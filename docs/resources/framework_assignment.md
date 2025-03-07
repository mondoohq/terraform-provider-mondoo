---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "mondoo_framework_assignment Resource - terraform-provider-mondoo"
subcategory: ""
description: |-
  Set compliance frameworks for a Mondoo space.
---

# mondoo_framework_assignment (Resource)

Set compliance frameworks for a Mondoo space.

## Example Usage

```terraform
provider "mondoo" {
  space = "hungry-poet-123456"
}

resource "mondoo_framework_assignment" "framework_assignment" {
  framework_mrn = [
    "//policy.api.mondoo.app/frameworks/cis-controls-8",
    "//policy.api.mondoo.app/frameworks/iso-27001-2022"
  ]
  enabled = true
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `enabled` (Boolean) Enable or disable the compliance framework.
- `framework_mrn` (List of String) Compliance framework MRN.

### Optional

- `space_id` (String) Mondoo space identifier. If there's no ID, the provider space is used.
