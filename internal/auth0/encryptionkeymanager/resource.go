package encryptionkeymanager

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/id"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/auth0/go-auth0"
	"github.com/auth0/go-auth0/management"

	"github.com/auth0/terraform-provider-auth0/internal/config"
	internalError "github.com/auth0/terraform-provider-auth0/internal/error"
	"github.com/auth0/terraform-provider-auth0/internal/value"
	"github.com/auth0/terraform-provider-auth0/internal/wait"
)

// NewEncryptionKeyManagerResource will return a new auth0_encryption_key_manager resource.
func NewEncryptionKeyManagerResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: createEncryptionKeyManager,
		UpdateContext: updateEncryptionKeyManager,
		ReadContext:   readEncryptionKeyManager,
		DeleteContext: deleteEncryptionKeyManager,
		Description:   "Resource to allow the rekeying of your tenant master key.",
		Schema: map[string]*schema.Schema{
			"key_rotation_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "If this value is changed, the encryption keys will be rotated. A UUID is recommended for the `key_rotation_id`.",
			},
			"customer_provided_root_key": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Description: "This attribute is used for provisioning the customer provided " +
					"root key. To initiate the provisioning process, create a new empty " +
					"`customer_provided_root_key` block. After applying this, the " +
					"`public_wrapping_key` can be retreived from the resource, and the new root " +
					"key should be generated by the customer and wrapped with the wrapping key, " +
					"then base64-encoded and added as the `wrapped_key` attribute.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"wrapped_key": {
							Type:     schema.TypeString,
							Optional: true,
							Description: "The base64-encoded customer provided root key, " +
								"wrapped using the `public_wrapping_key`. This can be removed " +
								"after the wrapped key has been applied.",
						},
						"public_wrapping_key": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The public wrapping key in PEM format.",
						},
						"wrapping_algorithm": {
							Type:     schema.TypeString,
							Computed: true,
							Description: "The algorithm that should be used to wrap the " +
								"customer provided root key. Should be `CKM_RSA_AES_KEY_WRAP`.",
						},
						"key_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The key ID of the customer provided root key.",
						},
						"type": {
							Type:     schema.TypeString,
							Computed: true,
							Description: "The type of the customer provided root key. " +
								"Should be `customer-provided-root-key`.",
						},
						"state": {
							Type:     schema.TypeString,
							Computed: true,
							Description: "The state of the encryption key. One of " +
								"`pre-activation`, `active`, `deactivated`, or `destroyed`.",
						},
						"parent_key_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The key ID of the parent wrapping key.",
						},
						"created_at": {
							Type:     schema.TypeString,
							Computed: true,
							Description: "The ISO 8601 formatted date the customer provided " +
								"root key was created.",
						},
						"updated_at": {
							Type:     schema.TypeString,
							Computed: true,
							Description: "The ISO 8601 formatted date the customer provided " +
								"root key was updated.",
						},
					},
				},
			},
			"encryption_keys": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "All encryption keys.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"key_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The key ID of the encryption key.",
						},
						"type": {
							Type:     schema.TypeString,
							Computed: true,
							Description: "The type of the encryption key. One of " +
								"`customer-provided-root-key`, `environment-root-key`, " +
								"or `tenant-master-key`.",
						},
						"state": {
							Type:     schema.TypeString,
							Computed: true,
							Description: "The state of the encryption key. One of " +
								"`pre-activation`, `active`, `deactivated`, or `destroyed`.",
						},
						"parent_key_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The key ID of the parent wrapping key.",
						},
						"created_at": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The ISO 8601 formatted date the encryption key was created.",
						},
						"updated_at": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The ISO 8601 formatted date the encryption key was updated.",
						},
					},
				},
			},
		},
	}
}

func createEncryptionKeyManager(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	data.SetId(id.UniqueId())

	return updateEncryptionKeyManager(ctx, data, meta)
}

func updateEncryptionKeyManager(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	api := meta.(*config.Config).GetAPI()
	config := data.GetRawConfig()

	if !data.IsNewResource() && data.HasChange("key_rotation_id") {
		keyRotationID := data.Get("key_rotation_id").(string)
		if len(keyRotationID) > 0 {
			if err := api.EncryptionKey.Rekey(ctx); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	if data.IsNewResource() || data.HasChange("customer_provided_root_key") {
		oldCountValue, newCountValue := data.GetChange("customer_provided_root_key.#")
		rootKeyID := data.Get("customer_provided_root_key.0.key_id").(string)
		rootKeyState := data.Get("customer_provided_root_key.0.state").(string)
		publicWrappingKey := data.Get("customer_provided_root_key.0.public_wrapping_key").(string)

		rootKeyAttrib := config.GetAttr("customer_provided_root_key")
		if rootKeyAttrib.IsNull() || rootKeyAttrib.LengthInt() == 0 {
			// The customer_provided_root_key block is not present, check if there was a key.
			if len(rootKeyID) > 0 {
				if err := removeKey(ctx, api, rootKeyID); err != nil {
					return diag.FromErr(err)
				}
			}
		} else {
			var wrappedKey *string

			rootKeyAttrib.ForEachElement(func(_ cty.Value, cfg cty.Value) (stop bool) {
				wrappedKey = value.String(cfg.GetAttr("wrapped_key"))
				return stop
			})
			if wrappedKey != nil {
				if len(rootKeyID) > 0 && rootKeyState == "pre-activation" && len(publicWrappingKey) > 0 {
					if err := importWrappedKey(ctx, api, auth0.String(rootKeyID), wrappedKey); err != nil {
						return diag.FromErr(err)
					}
				} else if len(rootKeyID) == 0 || len(publicWrappingKey) == 0 {
					return diag.FromErr(fmt.Errorf("The wrapped_key attribute should not be specified in the " +
						"customer_provided_root_key block until after the public_wrapping_key has been generated"))
				}
			}

			// If we don't have a root key in progress yet, or this block is newly created
			// create a new one.
			if len(rootKeyID) == 0 || (oldCountValue.(int) == 0 && newCountValue.(int) == 1) {
				if rootKey, wrappingKey, err := createRootKey(ctx, api); err != nil {
					return diag.FromErr(err)
				} else if err := data.Set("customer_provided_root_key", flattenCustomerProvidedRootKey(data, rootKey, wrappingKey)); err != nil {
					return diag.FromErr(err)
				}
			}
		}
	}

	return readEncryptionKeyManager(ctx, data, meta)
}

func readEncryptionKeyManager(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	api := meta.(*config.Config).GetAPI()

	encryptionKeys := make([]*management.EncryptionKey, 0)
	page := 0
	for {
		encryptionKeyList, err := api.EncryptionKey.List(ctx, management.Page(page), management.PerPage(5))
		if err != nil {
			return diag.FromErr(err)
		}
		encryptionKeys = append(encryptionKeys, encryptionKeyList.Keys...)
		if !encryptionKeyList.HasNext() {
			break
		}
		page++
	}

	if data.Get("customer_provided_root_key.#").(int) > 0 {
		// First try to find a key that is going through the activation process.
		rootKey := getKeyByTypeAndState("customer-provided-root-key", "pre-activation", encryptionKeys)

		if rootKey == nil {
			// If we didn't find one, try to find a key that is already active.
			rootKey = getKeyByTypeAndState("customer-provided-root-key", "active", encryptionKeys)
		}

		if rootKey != nil {
			if err := data.Set("customer_provided_root_key", flattenCustomerProvidedRootKey(data, rootKey, nil)); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	return diag.FromErr(data.Set("encryption_keys", flattenEncryptionKeys(encryptionKeys)))
}

func deleteEncryptionKeyManager(ctx context.Context, data *schema.ResourceData, meta interface{}) diag.Diagnostics {
	api := meta.(*config.Config).GetAPI()

	rootKeyID := data.Get("customer_provided_root_key.0.key_id").(string)
	if len(rootKeyID) > 0 {
		return diag.FromErr(removeKey(ctx, api, rootKeyID))
	}

	return nil
}

func removeKey(ctx context.Context, api *management.Management, keyID string) error {
	if err := api.EncryptionKey.Delete(ctx, keyID); err != nil {
		return err
	}

	// Wait until the key is actually destroyed.
	return wait.Until(100, 20, func() (bool, error) {
		key, err := api.EncryptionKey.Read(ctx, keyID)
		if err != nil {
			return false, err
		}
		return key.GetState() == "destroyed", nil
	})
}

func importWrappedKey(ctx context.Context, api *management.Management, keyID, wrappedKey *string) error {
	encryptionKey := management.EncryptionKey{
		KID:        keyID,
		WrappedKey: wrappedKey,
	}
	if err := api.EncryptionKey.ImportWrappedKey(ctx, &encryptionKey); err != nil {
		return err
	}
	// Wait until the key is actually activated.
	return wait.Until(100, 20, func() (bool, error) {
		key, err := api.EncryptionKey.Read(ctx, *keyID)
		if err != nil {
			return false, err
		}
		return key.GetState() == "active", nil
	})
}

func createRootKey(ctx context.Context, api *management.Management) (*management.EncryptionKey, *management.WrappingKey, error) {
	key := management.EncryptionKey{
		Type: auth0.String("customer-provided-root-key"),
	}
	if err := api.EncryptionKey.Create(ctx, &key); err != nil {
		return nil, nil, err
	}

	// Wait until the key is actually available.
	err := wait.Until(100, 20, func() (bool, error) {
		if _, err := api.EncryptionKey.Read(ctx, key.GetKID()); err != nil {
			if internalError.IsStatusNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return true, nil
	})
	if err != nil {
		return nil, nil, err
	}

	wrappingKey, err := api.EncryptionKey.CreatePublicWrappingKey(ctx, key.GetKID())
	if err != nil {
		return nil, nil, err
	}

	return &key, wrappingKey, nil
}