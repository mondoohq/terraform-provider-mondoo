variable "mondoo_org" {
  description = "The Mondoo Organization ID"
  type        = string
}

provider "mondoo" {
  region = "us"
}

# Create a new space
resource "mondoo_space" "domain_space" {
  name   = "My Space Name"
  org_id = var.mondoo_org
}

# Setup the Domain integration
resource "mondoo_integration_domain" "domain_integration" {
  space_id = mondoo_space.domain_space.id
  host     = "mondoo.com"
  https    = true
  http     = false
}