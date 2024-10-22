provider "mondoo" {
  space = "hungry-poet-123456"
}

resource "mondoo_policy_assignment" "space" {
  policies = [
    "//policy.api.mondoo.app/policies/mondoo-aws-security",
  ]
}
