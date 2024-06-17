provider "mondoo" {
  region = "us"
}

variable "mondoo_org" {
  description = "The Mondoo Organization ID"
  type        = string
  default     = "my-org-1234567"
}

# Create a new space
resource "mondoo_space" "my_space" {
  name   = "Framework Space"
  org_id = var.mondoo_org
}

resource "mondoo_framework_assignment" "compliance_framework_example" {
  space_id = mondoo_space.my_space.id
  framework_mrn = ["//policy.api.mondoo.app/frameworks/cis-controls-8",
  "//policy.api.mondoo.app/frameworks/iso-27001-2022"]
  enabled = true
}
