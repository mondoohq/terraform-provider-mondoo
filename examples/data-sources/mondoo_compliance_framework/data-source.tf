data "mondoo_compliance_framework" "frameworks" {
  space_id = "your-space-1234567"
  # space_mrn    = "your-space-mrn" 
}

output "framework_mrn" {
  value       = [for framework in data.mondoo_compliance_framework.frameworks.compliance_frameworks : framework.mrn]
  description = "The MRN of the frameworks in the space."
}
