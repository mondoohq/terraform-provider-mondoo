variable "org_id" {
  description = "The ID of the organization in which to create the space and teams"
  type        = string
}

provider "mondoo" {}

# Get the current organization data
data "mondoo_organization" "current" {
  id = var.org_id
}

# Create a space for our teams
resource "mondoo_space" "my_space" {
  name        = "my-space"
  description = "My Space"
  org_id      = var.org_id
}

# Create a team scoped to the space
resource "mondoo_team" "team_1" {
  name        = "team-1"
  description = "Team 1"
  scope_mrn   = mondoo_space.my_space.mrn
}

# Create another team scoped to the organization
resource "mondoo_team" "team_2" {
  name        = "team-2"
  description = "Team 2"
  scope_mrn   = data.mondoo_organization.current.mrn
}

resource "mondoo_team_external_group_mapping" "team_2" {
  team_mrn    = mondoo_team.team_2.mrn
  external_id = "team2"
}

# Example of team with IAM permissions (using existing mondoo_iam_binding resource)
# This would give the security team editor permissions on their space
resource "mondoo_iam_binding" "security_team_permissions" {
  identity_mrn = mondoo_team.team_1.mrn
  resource_mrn = mondoo_space.my_space.mrn
  roles        = ["//iam.api.mondoo.app/roles/editor"]
}

# Example output to show team details
output "team1" {
  value       = mondoo_team.team_1
  description = "Team 1"
}

output "team2" {
  value       = mondoo_team.team_2
  description = "Team 2"
}

