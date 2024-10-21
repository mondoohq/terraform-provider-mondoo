variable "slack_token" {
  description = "The Slack Token"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the Slack integration
resource "mondoo_integration_slack" "slack_integration" {
  name        = "My Slack Integration"
  slack_token = var.slack_token
}
