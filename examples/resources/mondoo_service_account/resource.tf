provider "mondoo" {
  space = "hungry-poet-123456"
}

resource "mondoo_service_account" "service_account" {
  name        = "Service Account Terraform"
  description = "Service Account for Terraform"
  roles = [
    "//iam.api.mondoo.app/roles/viewer",
  ]
}

output "service_account_json" {
  description = "Service Account as JSON"
  value       = base64decode(mondoo_service_account.service_account.credential)
  sensitive   = true
}

output "service_account_base64" {
  description = "Service Account as Base64"
  value       = mondoo_service_account.service_account.credential
  sensitive   = true
}

