variable "client_id" {
  description = "The Client ID"
  type        = string
  sensitive   = true
}

variable "client_secret" {
  description = "The Client Secret"
  type        = string
  sensitive   = true
}

variable "cloud" {
  description = "The Falcon Cloud to connect"
  type        = string
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Set up the CrowdStrike integration
resource "mondoo_integration_crowdstrike" "crowdstrike_integration" {
  name          = "CrowdStrike Integration"
  client_id     = var.client_id
  client_secret = var.client_secret
  cloud         = var.cloud

  # Optional: also import Falcon alerts and quarantined files (needs the
  # Alerts: Read and Quarantined Files: Read API scopes). Spotlight
  # vulnerabilities are always imported.
  finding_types = ["VULNERABILITY", "THREAT"]

  # Optional: import only these severities. Omit or leave empty for all.
  severities = ["CRITICAL", "HIGH"]
}
