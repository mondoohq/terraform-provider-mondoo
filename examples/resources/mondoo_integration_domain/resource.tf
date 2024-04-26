terraform {
  required_providers {
    mondoo = {
      source = "mondoohq/mondoo"
    }
  }
}

provider "mondoo" {
  region = "us"
}

// Create a new space
resource "mondoo_space" "domain_space" {
  name   = "My Space Name"
  org_id = "your-org-1234567"
}

// Setup the Domain integration
resource "mondoo_integration_domain" "domain_integration" {
  space_id = mondoo_space.domain_space.id
  host     = "example.com"
  https    = true
  http     = false
}