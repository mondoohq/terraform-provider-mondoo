output "space_id" {
  description = "ID of the application team's space."
  value       = mondoo_space.app.id
}

output "team_mrns" {
  description = "MRNs of the teams, to use in mondoo_iam_binding elsewhere."
  value = {
    security = mondoo_team.security.mrn
    app      = mondoo_team.app.mrn
  }
}

output "reporting_service_account" {
  description = "Service account credentials, base64 encoded. Pass them to the job as MONDOO_CONFIG_BASE64."
  value       = mondoo_service_account.reporting.credential
  sensitive   = true
}
