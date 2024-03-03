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

resource "mondoo_space" "test" {
  org_id = mondoo_organization.org.id
  name   = "test-space"
}

data "mondoo_space" "space" {
  id = mondoo_space.test.id

  depends_on = [
    mondoo_space.test
  ]
}

output "space_name" {
  value = data.mondoo_space.name
}