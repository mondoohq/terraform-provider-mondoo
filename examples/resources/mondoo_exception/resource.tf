provider "mondoo" {
  space = "eu-practical-goldwasser-115737"
}

resource "mondoo_exception" "exception" {
  scope_mrn = "//assets.api.mondoo.app/spaces/eu-practical-goldwasser-115737/assets/2phR0MlEtyxv1kknyk7nNXYjddW"
  # valid_until = "2024-12-03"
  justification = "This is a test exception"
  action ="DISABLE"
  # check_mrns = ["//policy.api.mondoo.app/queries/mondoo-http-security-x-content-type-options-nosniff"]
  check_mrns = ["//policy.api.mondoo.app/queries/mondoo-http-security-x-content-type-options-nosniff"]
}
