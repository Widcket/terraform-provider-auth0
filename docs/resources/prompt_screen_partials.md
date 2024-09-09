---
page_title: "Resource: auth0_prompt_screen_partials"
description: |-
  With this resource, you can manage a customized sign up and login experience by adding custom content, form elements and css/javascript. You can read more about this here https://auth0.com/docs/customize/universal-login-pages/customize-signup-and-login-prompts.
---

# Resource: auth0_prompt_screen_partials

With this resource, you can manage a customized sign up and login experience by adding custom content, form elements and css/javascript. You can read more about this [here](https://auth0.com/docs/customize/universal-login-pages/customize-signup-and-login-prompts).

!> This resource manages the entire set of prompt screens enabled for a prompt. In contrast, the `auth0_prompt_screen_partial`
resource appends a specific prompt screen to the list of prompt screens displayed to the user during the authentication flow.
 To avoid potential issues, it is recommended not to use this resource in conjunction with the `auth0_prompt_screen_partial`
 resource when managing prompt screens for the same prompt.

## Example Usage

```terraform
resource "auth0_prompt_screen_partials" "prompt_screen_partials" {
  prompt_type = "login-passwordless"

  screen_partials {
    screen_name = "login-passwordless-email-code"
    insertion_points {
      form_content_start = "<div>Form Content Start</div>"
      form_content_end   = "<div>Form Content End</div>"
    }
  }

  screen_partials {
    screen_name = "login-passwordless-sms-otp"
    insertion_points {
      form_content_start = "<div>Form Content Start</div>"
      form_content_end   = "<div>Form Content End</div>"
    }
  }
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `prompt_type` (String) The prompt that you are adding partials for. Options are: `login-id`, `login`, `login-password`, `signup`, `signup-id`, `signup-password`, `login-passwordless`.

### Optional

- `screen_partials` (Block List) (see [below for nested schema](#nestedblock--screen_partials))

### Read-Only

- `id` (String) The ID of this resource.

<a id="nestedblock--screen_partials"></a>
### Nested Schema for `screen_partials`

Required:

- `insertion_points` (Block List, Min: 1, Max: 1) (see [below for nested schema](#nestedblock--screen_partials--insertion_points))
- `screen_name` (String) The name of the screen associated with the partials

<a id="nestedblock--screen_partials--insertion_points"></a>
### Nested Schema for `screen_partials.insertion_points`

Optional:

- `form_content_end` (String) Content that goes at the end of the form.
- `form_content_start` (String) Content that goes at the start of the form.
- `form_footer_end` (String) Footer content for the end of the footer.
- `form_footer_start` (String) Footer content for the start of the footer.
- `secondary_actions_end` (String) Actions that go at the end of secondary actions.
- `secondary_actions_start` (String) Actions that go at the start of secondary actions.

## Import

Import is supported using the following syntax:

```shell
# This resource can be imported using the prompt name.
#
# Example:
terraform import auth0_prompt_screen_partials.prompt_screen_partials "login-passwordless"
```