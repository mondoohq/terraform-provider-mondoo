terraform {
  required_providers {
    mondoo = {
      source  = "mondoo/mondoo"
    }
  }
}

provider "mondoo" {
  region = "us"
}

resource "mondoo_space" "my_space" {
  name     = "My Space Name"
  space_id = "your-space-id" # optional
  org_id   = "your-org-1234567"
}
