variable "org_id" {
  description = "The ID of the organization in which to create the space and teams"
  type        = string
}

provider "mondoo" {}

data "mondoo_organization" "example" {
  id = var.org_id
}

# Add a member to a team by email
resource "mondoo_team" "example" {
  name      = "security-team"
  scope_mrn = data.mondoo_organization.example.mrn
}

resource "mondoo_team_member" "alice" {
  team_mrn = mondoo_team.example.mrn
  identity = "alice@example.com"
}

resource "mondoo_team_member" "bob" {
  team_mrn = mondoo_team.example.mrn
  identity = "bob@example.com"
}

