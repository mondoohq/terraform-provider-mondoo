terraform {
  required_providers {
    mondoo = {
      source = "mondoohq/mondoo"
    }
  }
}

provider "mondoo" {

}

resource "mondoo_space" "my_space" {
  name   = "My Custom Space"
  org_id = "your-org-1234567"
}

variable "my_custom_policy" {
  type    = string
  default = "policy.mql.yaml"
}

resource "mondoo_custom_policy" "my_policy" {
  space_id  = mondoo_space.my_space.id
  source    = var.my_custom_policy
  overwrite = true
}

resource "mondoo_policy_assignment" "space" {
  space_id = mondoo_space.my_space.id

  policies = concat(
    mondoo_custom_policy.my_policy.mrns,
    [],
  )

  state = "enabled"

  depends_on = [
    mondoo_space.my_space
  ]
}