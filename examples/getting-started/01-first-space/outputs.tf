output "space_id" {
  description = "ID of the new space. Use it as the provider's space setting or as space_id on other resources."
  value       = mondoo_space.this.id
}

output "space_mrn" {
  description = "Mondoo Resource Name (MRN) of the new space. Resources that take a scope_mrn accept it."
  value       = mondoo_space.this.mrn
}

output "registration_token" {
  description = "Token that registers a machine in the space. Read it with: terraform output -raw registration_token"
  value       = mondoo_registration_token.scan.result
  sensitive   = true
}
