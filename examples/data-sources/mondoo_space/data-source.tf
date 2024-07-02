variable "mondoo_org" {
  description = "Mondoo Organization"
  type        = string
}

provider "mondoo" {}

resource "mondoo_space" "test" {
  org_id = var.mondoo_org
  name   = "test-space"
}

data "mondoo_space" "space" {
  id = mondoo_space.test.id

  depends_on = [
    mondoo_space.test
  ]
}

output "space_name" {
  description = "The name of the space"
  value       = data.mondoo_space.space.name
}

output "space_mrn" {
  description = "The MRN of the space"
  value       = data.mondoo_space.space.mrn
}

output "space_id" {
  description = "The ID of the space"
  value       = data.mondoo_space.space.id
}
