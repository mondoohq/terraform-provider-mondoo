variable "client_secret" {
  description = "Client secret of the Microsoft Entra app registration"
  type        = string
  sensitive   = true
}

# Scan every repository of an Azure DevOps organization, from the Mondoo organization
resource "mondoo_integration_azure_devops_repos" "organization" {
  org_id               = "your-org-1234567"
  name                 = "Azure DevOps: acme"
  organization         = "acme"
  repository_deny_list = ["archive-*"]
  secret_scan          = true
  vulnerability_scan   = true

  credentials = {
    tenant_id     = "ffffffff-ffff-ffff-ffff-ffffffffffff"
    client_id     = "ffffffff-ffff-ffff-ffff-ffffffffffff"
    client_secret = var.client_secret
  }
}

# Scan one repository, from a space, authenticating with a certificate
resource "mondoo_integration_azure_devops_repos" "repository" {
  space_id     = "hungry-poet-123456"
  name         = "Azure DevOps: acme/web/api"
  organization = "acme"
  project      = "web"
  repository   = "api"
  code_scan    = true

  credentials = {
    tenant_id   = "ffffffff-ffff-ffff-ffff-ffffffffffff"
    client_id   = "ffffffff-ffff-ffff-ffff-ffffffffffff"
    certificate = file("${path.module}/mondoo-ado.pem")
  }
}
