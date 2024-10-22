provider "mondoo" {
  space = "hungry-poet-123456"
}

variable "my_custom_framework" {
  description = "Path to the custom policy file. The file must be in MQL format."
  type        = string
  default     = "framework.mql.yaml"
}

resource "mondoo_custom_framework" "custom_framework" {
  data_url = var.my_custom_framework
}
