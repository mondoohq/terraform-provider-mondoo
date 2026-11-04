# Workload Identity Federation (keyless), in the Mondoo organization
# -----------------------------------------------
#
# The integration lives in the Mondoo organization instead of a space. The API
# creates no routing rule: the organization's asset routing rules place the
# assets it discovers into spaces.
#
# The WIF subject of an organization integration names the organization, not a
# space. Always wire up the federated identity credential from the integration's
# wif_subject and wif_issuer_url; never build the subject yourself.

variable "tenant_id" {
  description = "The Azure Active Directory Tenant ID"
  type        = string
  default     = "ffffffff-ffff-ffff-ffff-ffffffffffff"
}

variable "application_object_id" {
  description = "Object ID of the Azure AD application Mondoo authenticates as"
  type        = string
}

variable "application_client_id" {
  description = "Application (client) ID of the Azure AD application Mondoo authenticates as"
  type        = string
}

provider "azuread" {
  tenant_id = var.tenant_id
}

provider "mondoo" {}

resource "mondoo_integration_azure" "organization" {
  org_id    = "your-org-1234567"
  name      = "Azure (WIF)"
  tenant_id = var.tenant_id
  client_id = var.application_client_id
  use_wif   = true
}

resource "azuread_application_federated_identity_credential" "mondoo_organization" {
  application_id = "/applications/${var.application_object_id}"
  display_name   = "mondoo-organization"
  issuer         = mondoo_integration_azure.organization.wif_issuer_url
  subject        = mondoo_integration_azure.organization.wif_subject
  audiences      = ["api://AzureADTokenExchange"]
}

# Route every asset of this integration into one space
resource "mondoo_asset_routing_rule" "azure" {
  org_mrn          = "//captain.api.mondoo.app/organizations/your-org-1234567"
  target_space_mrn = "//captain.api.mondoo.app/spaces/hungry-poet-123456"
  priority         = 10

  condition {
    field    = "LABEL"
    operator = "EQUAL"
    key      = "mondoo.com/integration-mrn"
    values   = [mondoo_integration_azure.organization.mrn]
  }
}
