terraform {
  required_providers {
    mondoo = {
      source = "mondoohq/mondoo"
    }
  }
}

provider "mondoo" {
  region = "us"
}

resource "mondoo_space" "my_space" {
  name   = "My Space Name"
  org_id = "your-org-1234567"
}

variable "my_custom_querypack" {
  type    = string
  default = "/path/to/my-custom-policy.mql.yml"
}

resource "mondoo_custom_querypack" "my_query_pack" {
  space_id = mondoo_space.my_space.id
  source   = var.my_custom_querypack
}

resource "mondoo_querypack_assignments" "space" {
  space_id = mondoo_space.my_space.id

  policies = [
    mondoo_custom_querypack.my_query_pack.mrn # use a uploaded policy mrn
  ]

  state = "enabled"

  depends_on = [
    mondoo_space.my_space
  ]
}