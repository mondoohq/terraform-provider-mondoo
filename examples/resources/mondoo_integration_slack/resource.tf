variable "mondoo_org" {
  description = "The Mondoo Organization ID"
  type        = string
}

variable "slack_token" {
  description = "The Slack Token"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  region = "us"
}

# Create a new space
resource "mondoo_space" "my_space" {
  name   = "My Slack Space"
  org_id = var.mondoo_org
}

# Setup the Slack integration
resource "mondoo_integration_slack" "slack_integration" {
  space_id = mondoo_space.my_space.id
  name     = "My Slack Integration"

  slack_token = var.slack_token
}
