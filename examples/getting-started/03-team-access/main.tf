data "mondoo_organization" "this" {
  id = var.org_id
}

# The space the application team works in.
resource "mondoo_space" "app" {
  org_id      = var.org_id
  name        = "Payments application"
  description = "Created by the Mondoo Terraform team-access example."
}

# Teams group people so you grant access once per team instead of once per
# person. A team belongs to an organization or a space (scope_mrn), and roles
# are granted separately with mondoo_iam_binding.
resource "mondoo_team" "security" {
  name        = "security"
  description = "Owns policies and reviews exceptions across the organization."
  scope_mrn   = data.mondoo_organization.this.mrn
}

resource "mondoo_team" "app" {
  name        = "payments-app"
  description = "Builds and runs the payments application."
  scope_mrn   = mondoo_space.app.mrn
}

# Members are added by email address. for_each over a set means removing an
# address from the variable removes only that person.
resource "mondoo_team_member" "security" {
  for_each = toset(var.security_team_members)

  team_mrn = mondoo_team.security.mrn
  identity = each.value
}

resource "mondoo_team_member" "app" {
  for_each = toset(var.app_team_members)

  team_mrn = mondoo_team.app.mrn
  identity = each.value
}

# With SSO, map an identity provider group to the team instead of listing
# members. Created only when security_team_idp_group is set.
resource "mondoo_team_external_group_mapping" "security" {
  count = var.security_team_idp_group == null ? 0 : 1

  team_mrn    = mondoo_team.security.mrn
  external_id = var.security_team_idp_group
}

# Roles, from least to most access: viewer, editor, owner. Narrower roles such
# as policy-manager or exceptions-requester grant a single capability. See the
# mondoo_iam_binding documentation for the full list.

# The security team manages policies everywhere in the organization and can
# see every space.
resource "mondoo_iam_binding" "security_org" {
  identity_mrn = mondoo_team.security.mrn
  resource_mrn = data.mondoo_organization.this.mrn
  roles        = ["viewer", "policy-manager"]
}

# The application team sees its own space and can request exceptions, which
# someone with more access approves.
resource "mondoo_iam_binding" "app_space" {
  identity_mrn = mondoo_team.app.mrn
  resource_mrn = mondoo_space.app.mrn
  roles        = ["viewer", "exceptions-requester"]
}

# Automation authenticates with a service account, not a person's account.
# This one can read the space, for example for a job that exports reports.
resource "mondoo_service_account" "reporting" {
  space_id    = mondoo_space.app.id
  name        = "reporting"
  description = "Read-only access for the weekly security report job."
  roles       = ["//iam.api.mondoo.app/roles/viewer"]
}
