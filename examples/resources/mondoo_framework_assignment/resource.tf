provider "mondoo" {
  space = "hungry-poet-123456"
}

# Enable compliance frameworks in the provider's space. A framework maps the
# results of your policies to its controls, so you can track progress toward
# each control in the Mondoo Console.
resource "mondoo_framework_assignment" "compliance" {
  framework_mrn = [
    "//policy.api.mondoo.app/frameworks/cis-controls-8",
    "//policy.api.mondoo.app/frameworks/iso-27001-2022",
  ]
  enabled = true
}

# To see which frameworks a space has, use the mondoo_frameworks data source.
