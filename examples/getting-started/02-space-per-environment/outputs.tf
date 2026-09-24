output "spaces" {
  description = "ID and MRN of each environment's space, keyed by environment name."
  value = {
    for name, space in mondoo_space.env : name => {
      id  = space.id
      mrn = space.mrn
    }
  }
}

output "registration_tokens" {
  description = "Registration token for each environment. Read one with: terraform output -json registration_tokens | jq -r .production"
  value       = { for name, token in mondoo_registration_token.env : name => token.result }
  sensitive   = true
}
