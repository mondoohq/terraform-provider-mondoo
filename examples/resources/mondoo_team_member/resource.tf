# Add a member to a team by email
resource "mondoo_team" "example" {
  name      = "security-team"
  scope_mrn = mondoo_organization.example.mrn
}

resource "mondoo_team_member" "alice" {
  team_mrn = mondoo_team.example.mrn
  email    = "alice@example.com"
}

resource "mondoo_team_member" "bob" {
  team_mrn = mondoo_team.example.mrn
  email    = "bob@example.com"
}
