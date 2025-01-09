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

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the CrowdStrike integration
resource "mondoo_integration_crowdstrike" "crowdstrike_integration" {
  name          = "CrowdStrike Integration"
  client_id     = var.client_id
  client_secret = var.client_secret
}
