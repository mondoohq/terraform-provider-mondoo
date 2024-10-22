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
