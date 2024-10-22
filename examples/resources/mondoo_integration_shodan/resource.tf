variable "mondoo_org" {
  description = "The Mondoo Organization ID"
  type        = string
}

variable "shodan_token" {
  description = "The Shodan Token"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  region = "us"
}

# Create a new space
resource "mondoo_space" "shodan_space" {
  name   = "My Shodan Space Name"
  org_id = var.mondoo_org
}

# Setup the Shodan integration
resource "mondoo_integration_shodan" "shodan_integration" {
  space_id = mondoo_space.shodan_space.id
  name     = "Shodan Integration"

  targets = ["8.8.8.8", "mondoo.com"]

  credentials = {
    token = var.shodan_token
  }
}
