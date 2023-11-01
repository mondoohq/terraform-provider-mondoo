terraform {
  required_providers {
    mondoo = {
      source = "mondoo/mondoo"
    }
  }
}

provider "mondoo" {
  region = "us"
}

resource "mondoo_space" "my_space" {
  name = "My Space Name"
  # space_id = "your-space-id" # optional
  org_id   = "your-org-id"
}

resource "mondoo_registration_token" "token" {
  description = "Service Account for Terraform"
  space_id = mondoo_space.my_space.id
  no_exipration = true 
  // expires_in = "1h"
  depends_on = [
    mondoo_space.my_space
  ]
}

output "generated_token" {
  value = mondoo_registration_token.token.result
  sensitive = true
}