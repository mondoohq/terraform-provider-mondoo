# Variables
# ----------------------------------------------

variable "mondoo_org" {
  description = "The Mondoo Organization ID"
  type        = string
}

# Configure the Mondoo
# ----------------------------------------------

provider "mondoo" {
  region = "us"
}

resource "mondoo_space" "my_space" {
  name   = "My Terraform Space"
  org_id = var.mondoo_org
}

resource "mondoo_service_account" "service_account" {
  name        = "Service Account Terraform"
  description = "Service Account for Terraform"
  roles = [
    "//iam.api.mondoo.app/roles/viewer",
  ]
  space_id = mondoo_space.my_space.id

  depends_on = [
    mondoo_space.my_space
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

