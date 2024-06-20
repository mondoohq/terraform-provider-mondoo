data "mondoo_policy" "policy" {
  space_id      = "your-space-1234567"
  catalog_type  = "ALL" # availabe options: "ALL", "POLICY", "QUERYPACK"
  assigned_only = true
}

output "policies" {
  value = data.mondoo_policy.policy.policies.*.policy_mrn
}
