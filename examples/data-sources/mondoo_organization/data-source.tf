provider "mondoo" {}

data "mondoo_organization" "org" {
  id = "your-org-1234567"
}

output "org_mrn" {
  description = "MRN of the organization"
  value       = data.mondoo_organization.org.mrn
}

output "org_name" {
  description = "Name of the organization"
  value       = data.mondoo_organization.org.name
}

output "org_id" {
  description = "ID of the organization"
  value       = data.mondoo_organization.org.id
}
