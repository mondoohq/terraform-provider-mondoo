terraform {
  required_providers {
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.27"
    }
  }
}

provider "mondoo" {}

resource "mondoo_space" "new_space" {
    name        = "TF Provider Test Space"
    description = "A space created when testing the terraform provider (https://github.com/mondoohq/terraform-provider-mondoo)"
    org_id      = "relaxed-khayyam-961566"
}