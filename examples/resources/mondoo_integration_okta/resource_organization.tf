# Create the integration in the Mondoo organization instead of a space. The API
# creates no routing rule: the organization's asset routing rules place the
# assets it discovers into spaces.

variable "okta_token" {
  description = "The Okta API token"
  type        = string
  sensitive   = true
}

provider "mondoo" {}

resource "mondoo_integration_okta" "organization" {
  org_id       = "your-org-1234567"
  name         = "Okta Integration"
  organization = "acme.okta.com"
  token        = var.okta_token
}

# Route every asset of this integration into one space
resource "mondoo_asset_routing_rule" "okta" {
  org_mrn          = "//captain.api.mondoo.app/organizations/your-org-1234567"
  target_space_mrn = "//captain.api.mondoo.app/spaces/hungry-poet-123456"
  priority         = 10

  condition {
    field    = "LABEL"
    operator = "EQUAL"
    key      = "mondoo.com/integration-mrn"
    values   = [mondoo_integration_okta.organization.mrn]
  }
}
