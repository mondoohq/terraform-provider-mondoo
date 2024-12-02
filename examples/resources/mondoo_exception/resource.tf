provider "mondoo" {
  space = "eu-heuristic-hawking-332652"
}

resource "mondoo_exception" "exception" {
  # valid_until = "2024-12-03"
  justification = "This is a test exception"
  action ="DISABLE"
  # check_mrns = ["//policy.api.mondoo.app/queries/mondoo-http-security-x-content-type-options-nosniff"]
  check_mrns = ["//policy.api.mondoo.app/queries/mondoo-http-security-content-security-policy"]
}
