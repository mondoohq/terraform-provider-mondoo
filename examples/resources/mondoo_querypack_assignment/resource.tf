provider "mondoo" {
  space = "hungry-poet-123456"
}

resource "mondoo_querypack_assignment" "space" {
  querypacks = [
    "//policy.api.mondoo.app/policies/mondoo-incident-response-aws",
  ]
}
