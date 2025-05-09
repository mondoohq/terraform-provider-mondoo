
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
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the GoogleWorkspace integration
resource "mondoo_integration_google_workspace" "example" {
  name                    = "GoogleWorkspace Integration"
  foo                     = var.foo
  customer_id             = var.customer_id
  impersonated_user_email = var.impersonated_user_email
  service_account         = var.service_account
}
