variable "org_id" {
  description = "The ID of the organization"
  type        = string
}

provider "mondoo" {}

resource "mondoo_space" "example" {
  org_id = var.org_id
  name   = "Production"
}

resource "mondoo_team" "ops" {
  name      = "ops-team"
  scope_mrn = mondoo_space.example.mrn
  email     = "ops@example.com"
}

# Manage contacts for the space
resource "mondoo_resource_contacts" "example" {
  resource_mrn = mondoo_space.example.mrn
  contacts = [
    mondoo_team.ops.mrn,
    "security@example.com",
  ]
}
