variable "my_custom_querypack" {
  description = "Path to custom querypack file. File must be in MQL format."
  type        = string
  default     = "querypack.mql.yaml"
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

resource "mondoo_custom_querypack" "my_query_pack" {
  source = var.my_custom_querypack
}

resource "mondoo_querypack_assignment" "space" {
  querypacks = concat(
    mondoo_custom_querypack.my_query_pack.mrns,
    [],
  )

  state = "enabled"
}
