provider "mondoo" {}

data "mondoo_organization" "org" {
  id = "reverent-ride-275852"
}

output "org_mrn" {
  description = "MRN of the organization"
  value       = data.mondoo_organization.org.mrn
}