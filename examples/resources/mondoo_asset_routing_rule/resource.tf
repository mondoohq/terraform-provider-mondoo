# Manage individual routing rules independently.
# Ideal for multi-team setups where each team manages their own rules.

resource "mondoo_asset_routing_rule" "production" {
  org_mrn          = "//captain.api.mondoo.app/organizations/my-org-id"
  target_space_mrn = "//captain.api.mondoo.app/spaces/prod-space"
  priority         = 10

  condition {
    field    = "LABEL"
    operator = "EQUAL"
    key      = "env"
    values   = ["production"]
  }
}

resource "mondoo_asset_routing_rule" "staging" {
  org_mrn          = "//captain.api.mondoo.app/organizations/my-org-id"
  target_space_mrn = "//captain.api.mondoo.app/spaces/staging-space"
  priority         = 20

  condition {
    field    = "LABEL"
    operator = "EQUAL"
    key      = "env"
    values   = ["staging"]
  }
}
