provider "mondoo" {
  region = "us"
}

variable "mondoo_org" {
  description = "Mondoo Organization"
  type        = string
}

resource "mondoo_space" "my_space" {
  name   = "My Space Name"
  org_id = var.mondoo_org
}

variable "my_custom_querypack" {
  description = "Path to custom querypack file. File must be in MQL format."
  type        = string
  default     = "querypack.mql.yaml"
}

resource "mondoo_custom_querypack" "my_query_pack" {
  space_id = mondoo_space.my_space.id
  source   = var.my_custom_querypack
}

resource "mondoo_querypack_assignment" "space" {
  space_id = mondoo_space.my_space.id

  querypacks = concat(
    mondoo_custom_querypack.my_query_pack.mrns,
    [],
  )

  state = "enabled"

  depends_on = [
    mondoo_space.my_space
  ]
}