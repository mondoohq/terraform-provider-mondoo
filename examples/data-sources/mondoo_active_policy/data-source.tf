data "mondoo_active_policy" "policy" {
  space_id = "your-space-1234567"
}

output "policies_mrn" {
  value       = [for policy in data.mondoo_active_policy.policy.policies : policy.policy_mrn]
  description = "The MRN of the policies in the space"
}