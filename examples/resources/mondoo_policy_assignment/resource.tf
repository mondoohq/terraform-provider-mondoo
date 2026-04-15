# Assign policies to a space (using provider-configured space)
provider "mondoo" {
  space = "hungry-poet-123456"
}

resource "mondoo_policy_assignment" "space" {
  policies = [
    "//policy.api.mondoo.app/policies/mondoo-aws-security",
  ]
}

# Assign policies to an organization using scope_mrn
resource "mondoo_policy_assignment" "org" {
  scope_mrn = "//captain.api.mondoo.app/organizations/your-org-id"

  policies = [
    "//policy.api.mondoo.app/policies/mondoo-aws-security",
  ]
}
