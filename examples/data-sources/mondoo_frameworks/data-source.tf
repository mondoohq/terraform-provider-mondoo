data "mondoo_frameworks" "frameworks_data" {
  space_id = "your-space-1234567"
}

output "framework_mrn" {
  value       = [for framework in data.mondoo_frameworks.frameworks_data.frameworks : framework.mrn]
  description = "The MRN of the frameworks in the space."
}
