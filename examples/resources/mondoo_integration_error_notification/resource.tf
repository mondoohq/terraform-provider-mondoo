variable "shodan_token" {
  description = "The Shodan Token"
  type        = string
  sensitive   = true
}

variable "slack_webhook_url" {
  description = "Incoming-webhook URL of the Slack channel that receives the alert"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

resource "mondoo_integration_shodan" "shodan_integration" {
  name    = "Shodan Integration"
  targets = ["mondoo.com"]

  credentials = {
    token = var.shodan_token
  }
}

# Alert a Slack channel when the integration moves to an error state.
resource "mondoo_integration_error_notification" "shodan_alert" {
  integration_mrn = mondoo_integration_shodan.shodan_integration.mrn
  channel         = "SLACK"
  target          = var.slack_webhook_url
}

# Or send the alert by email.
resource "mondoo_integration_error_notification" "aws_alert" {
  integration_mrn = "//captain.api.mondoo.app/spaces/hungry-poet-123456/integrations/2Abd08lk860"
  channel         = "EMAIL"
  target          = "ops@example.com"
}
