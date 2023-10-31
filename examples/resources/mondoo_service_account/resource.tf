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
  org_id   = "your-org-1234567"
}

resource "mondoo_service_account" "service_account" {
  name        = "Service Account Terraform New"
  description = "Service Account for Terraform"
  roles = [
    "//iam.api.mondoo.app/roles/viewer", // TODO use "roles/viewer"
  ]
  space_id = mondoo_space.my_space.id

  depends_on = [
    mondoo_space.my_space
  ]
}
