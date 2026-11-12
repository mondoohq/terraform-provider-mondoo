# Create the integrations in the Mondoo organization instead of a space: one per
# Falcon tenant (CID). At organization scope the integration imports findings
# only; it does not run fleet scans. The API creates no routing rule: the
# organization's asset routing rules place each imported host into a space, and
# findings for a host no rule matches are not imported.

variable "falcon_eu_client_id" {
  description = "The Client ID of the EU Falcon tenant"
  type        = string
  sensitive   = true
}

variable "falcon_eu_client_secret" {
  description = "The Client Secret of the EU Falcon tenant"
  type        = string
  sensitive   = true
}

variable "falcon_us_client_id" {
  description = "The Client ID of the US Falcon tenant"
  type        = string
  sensitive   = true
}

variable "falcon_us_client_secret" {
  description = "The Client Secret of the US Falcon tenant"
  type        = string
  sensitive   = true
}

provider "mondoo" {}

resource "mondoo_integration_crowdstrike" "eu" {
  org_id        = "your-org-1234567"
  name          = "CrowdStrike EU"
  client_id     = var.falcon_eu_client_id
  client_secret = var.falcon_eu_client_secret
  cloud         = "eu-1"
}

resource "mondoo_integration_crowdstrike" "us" {
  org_id        = "your-org-1234567"
  name          = "CrowdStrike US"
  client_id     = var.falcon_us_client_id
  client_secret = var.falcon_us_client_secret
  cloud         = "us-1"
}

# Route each tenant's hosts into its own space
resource "mondoo_asset_routing_rule" "crowdstrike_eu" {
  org_mrn          = "//captain.api.mondoo.app/organizations/your-org-1234567"
  target_space_mrn = "//captain.api.mondoo.app/spaces/hungry-poet-123456"
  priority         = 10

  condition {
    field    = "LABEL"
    operator = "EQUAL"
    key      = "mondoo.com/integration-mrn"
    values   = [mondoo_integration_crowdstrike.eu.mrn]
  }
}

resource "mondoo_asset_routing_rule" "crowdstrike_us" {
  org_mrn          = "//captain.api.mondoo.app/organizations/your-org-1234567"
  target_space_mrn = "//captain.api.mondoo.app/spaces/happy-coder-654321"
  priority         = 10

  condition {
    field    = "LABEL"
    operator = "EQUAL"
    key      = "mondoo.com/integration-mrn"
    values   = [mondoo_integration_crowdstrike.us.mrn]
  }
}
