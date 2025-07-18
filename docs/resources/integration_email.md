---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "mondoo_integration_email Resource - terraform-provider-mondoo"
subcategory: ""
description: |-
  Send email to your ticket system or any recipient.
---

# mondoo_integration_email (Resource)

Send email to your ticket system or any recipient.

## Example Usage

```terraform
provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the Email integration
resource "mondoo_integration_email" "email_integration" {
  name = "My Email Integration"

  recipients = [
    {
      name          = "John Doe"
      email         = "john@example.com"
      is_default    = true
      reference_url = "https://example.com"
    },
    {
      name          = "Alice Doe"
      email         = "alice@example.com"
      is_default    = false
      reference_url = "https://example.com"
    }
  ]

  auto_create = true
  auto_close  = true
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Name of the integration.
- `recipients` (Attributes List) List of email recipients. (see [below for nested schema](#nestedatt--recipients))

### Optional

- `auto_close` (Boolean) Auto close tickets (defaults to false).
- `auto_create` (Boolean) Auto create tickets (defaults to false).
- `space_id` (String) Mondoo space identifier. If there is no space ID, the provider space is used.

### Read-Only

- `mrn` (String) Integration identifier

<a id="nestedatt--recipients"></a>
### Nested Schema for `recipients`

Required:

- `email` (String) Recipient email address.
- `name` (String) Recipient name.

Optional:

- `is_default` (Boolean) Mark this recipient as default. This must be set if auto_create is enabled.
- `reference_url` (String) Optional reference URL for the recipient.

## Import

Import is supported using the following syntax:

The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can be used, for example:

```shell
# Import using integration MRN.
terraform import mondoo_integration_email.email_integration "//captain.api.mondoo.app/spaces/hungry-poet-123456/integrations/2Abd08lk860"
```
