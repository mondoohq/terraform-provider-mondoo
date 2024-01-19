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

resource "mondoo_space" "my_space" {
  name = "My Space New"
  // id = "your-space-id" # optional otherwise it will be auto-generated
  org_id = "your-org-1234567"
}

