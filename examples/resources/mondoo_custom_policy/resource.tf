variable "my_custom_policy" {
  description = "Path to the custom policy file. The file must be in MQL format."
  type        = string
  default     = "policy.mql.yaml"
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

resource "mondoo_custom_policy" "my_policy" {
  source    = var.my_custom_policy
  overwrite = true
}

resource "mondoo_policy_assignment" "space" {
  policies = concat(
    mondoo_custom_policy.my_policy.mrns,
    [],
  )

  state = "enabled"
}
