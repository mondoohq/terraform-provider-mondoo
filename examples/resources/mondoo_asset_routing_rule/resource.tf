variable "org_id" {
  description = "The ID of the organization"
  type        = string
}

provider "mondoo" {}

data "mondoo_organization" "current" {
  id = var.org_id
}

# Create spaces for routing targets
resource "mondoo_space" "production" {
  name   = "production"
  org_id = var.org_id
}

resource "mondoo_space" "staging" {
  name   = "staging"
  org_id = var.org_id
}

# Manage individual routing rules independently.
# Ideal for multi-team setups where each team manages their own rules.
resource "mondoo_asset_routing_rule" "production" {
  org_mrn          = data.mondoo_organization.current.mrn
  target_space_mrn = mondoo_space.production.mrn
  priority         = 10

  condition {
    field    = "LABEL"
    operator = "EQUAL"
    key      = "env"
    values   = ["production"]
  }
}

resource "mondoo_asset_routing_rule" "staging" {
  org_mrn          = data.mondoo_organization.current.mrn
  target_space_mrn = mondoo_space.staging.mrn
  priority         = 20

  condition {
    field    = "LABEL"
    operator = "EQUAL"
    key      = "env"
    values   = ["staging"]
  }
}
