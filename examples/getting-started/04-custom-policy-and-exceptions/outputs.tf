output "space_id" {
  description = "ID of the new space."
  value       = mondoo_space.this.id
}

output "custom_policy_mrns" {
  description = "MRNs of the uploaded custom policies. Assign them in other spaces with mondoo_policy_assignment."
  value       = mondoo_custom_policy.ssh.mrns
}
