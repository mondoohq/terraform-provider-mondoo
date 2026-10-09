
variable "credential_mrn" {
  description = "The Okta CredentialMrn"
  type        = string
  default     = null
}
variable "organization" {
  description = "The Okta Organization"
  type        = string
  default     = null
}
variable "token" {
  description = "The Okta Token"
  type        = string
  default     = null
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Set up the Okta integration
resource "mondoo_integration_okta" "example" {
  name           = "Okta Integration"
  credential_mrn = var.credential_mrn
  organization   = var.organization
  token          = var.token
}
