variable "org_id" {
  description = "The ID of the organization"
  type        = string
}

provider "mondoo" {}

data "mondoo_organization" "current" {
  id = var.org_id
}

# Create spaces for routing targets
resource "mondoo_space" "linux" {
  name   = "linux-assets"
  org_id = var.org_id
}

resource "mondoo_space" "windows" {
  name   = "windows-assets"
  org_id = var.org_id
}

resource "mondoo_space" "catch_all" {
  name   = "catch-all"
  org_id = var.org_id
}

# Manage the entire routing table for an organization.
# Priority is derived from the order of rules (first = highest priority).
resource "mondoo_asset_routing_table" "example" {
  org_mrn = data.mondoo_organization.current.mrn

  # Rule 1: Route Linux assets
  rule {
    target_space_mrn = mondoo_space.linux.mrn

    condition {
      field    = "PLATFORM"
      operator = "EQUAL"
      values   = ["ubuntu", "debian", "rhel", "amazonlinux"]
    }
  }

  # Rule 2: Route Windows assets
  rule {
    target_space_mrn = mondoo_space.windows.mrn

    condition {
      field    = "PLATFORM"
      operator = "EQUAL"
      values   = ["windows"]
    }
  }

  # Rule 3: Catch-all for everything else
  rule {
    target_space_mrn = mondoo_space.catch_all.mrn
  }
}
