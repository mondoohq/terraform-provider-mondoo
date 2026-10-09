
variable "credential_mrn" {
  description = "The GoogleWorkspace CredentialMrn"
  type        = string
  default     = null
}
variable "customer_id" {
  description = "The GoogleWorkspace CustomerId"
  type        = string
}
variable "impersonated_user_email" {
  description = "The GoogleWorkspace ImpersonatedUserEmail"
  type        = string
}
variable "service_account" {
  description = "The GoogleWorkspace ServiceAccount"
  type        = string
  default     = null
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Set up the GoogleWorkspace integration
resource "mondoo_integration_google_workspace" "example" {
  name                    = "GoogleWorkspace Integration"
  credential_mrn          = var.credential_mrn
  customer_id             = var.customer_id
  impersonated_user_email = var.impersonated_user_email
  service_account         = var.service_account
}
