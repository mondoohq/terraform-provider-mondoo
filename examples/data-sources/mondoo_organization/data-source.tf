terraform {
  required_providers {
    mondoo = {
      source = "mondoohq/mondoo"
    }
  }
}

provider "mondoo" {
}

data "mondoo_organization" "org" {
  id = "reverent-ride-275852"
}

output "org_mrn" {
  value = data.mondoo_organization.org.mrn
}