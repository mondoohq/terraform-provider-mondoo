# Create a new organization. To work in an organization you already have,
# look it up with the mondoo_organization data source instead.
resource "mondoo_organization" "acme" {
  name        = "ACME Corp"
  description = "Security posture for all of ACME's infrastructure."
  company     = "ACME Corp"

  annotations = {
    cost-center = "security"
  }
}

# Spaces belong to an organization. Put the new space in the organization
# created above.
resource "mondoo_space" "production" {
  name   = "Production"
  org_id = mondoo_organization.acme.id
}
