
variable "auto_close_tickets" {
  description = "The AzureDevops AutoCloseTickets"
  type        = bool
}
variable "auto_create_tickets" {
  description = "The AzureDevops AutoCreateTickets"
  type        = bool
}
variable "client_secret" {
  description = "The AzureDevops ClientSecret"
  type        = string
}
variable "default_project_name" {
  description = "The AzureDevops DefaultProjectName"
  type        = string
}
variable "organization_url" {
  description = "The AzureDevops OrganizationUrl"
  type        = string
}
variable "service_principal_id" {
  description = "The AzureDevops ServicePrincipalId"
  type        = string
}
variable "tenant_id" {
  description = "The AzureDevops TenantId"
  type        = string
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the AzureDevops integration
resource "mondoo_integration_azure_devops" "example" {
  name                 = "AzureDevops Integration"
  auto_close_tickets   = var.auto_close_tickets
  auto_create_tickets  = var.auto_create_tickets
  client_secret        = var.client_secret
  default_project_name = var.default_project_name
  organization_url     = var.organization_url
  service_principal_id = var.service_principal_id
  tenant_id            = var.tenant_id
}
