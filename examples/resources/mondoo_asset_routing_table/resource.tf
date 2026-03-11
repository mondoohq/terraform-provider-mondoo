# Manage the entire routing table for an organization.
# Priority is derived from the order of rules (first = highest priority).
resource "mondoo_asset_routing_table" "example" {
  org_mrn = "//captain.api.mondoo.app/organizations/my-org-id"

  # Rule 1: Route Linux assets
  rule {
    target_space_mrn = "//captain.api.mondoo.app/spaces/linux-space"

    condition {
      field    = "PLATFORM"
      operator = "EQUAL"
      values   = ["ubuntu", "debian", "rhel", "amazonlinux"]
    }
  }

  # Rule 2: Route Windows assets
  rule {
    target_space_mrn = "//captain.api.mondoo.app/spaces/windows-space"

    condition {
      field    = "PLATFORM"
      operator = "EQUAL"
      values   = ["windows"]
    }
  }

  # Rule 3: Catch-all for everything else
  rule {
    target_space_mrn = "//captain.api.mondoo.app/spaces/catch-all-space"
  }
}
