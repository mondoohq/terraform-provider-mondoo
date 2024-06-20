data "mondoo_active_policy" "policy" {
  space_id = "your-space-1234567"
}

output "policies_mrn" {
  value = data.mondoo_active_policy.policy.policies.*.policy_mrn
}
