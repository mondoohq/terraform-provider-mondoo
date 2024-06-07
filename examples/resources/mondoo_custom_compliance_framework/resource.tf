provider "mondoo" {
  region = "us"
}

variable "mondoo_org" {
  description = "The Mondoo Organization ID"
  type        = string
  default     = "my-org-1234567"
}

variable "my_custom_framework" {
  description = "Path to the custom policy file. The file must be in MQL format."
  type        = string
  default     = "framework.mql.yaml"
}

# Create a new space
resource "mondoo_space" "my_space" {
  name   = "Custom Framework Space"
  org_id = var.mondoo_org
}

resource "mondoo_custom_compliance_framework" "compliance_framework_example" {
  space_id = mondoo_space.my_space.id
  data_url = var.my_custom_framework
}
