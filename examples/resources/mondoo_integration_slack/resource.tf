terraform {
  required_providers {
    mondoo = {
      source = "mondoohq/mondoo"
    }
  }
}

provider "mondoo" {
  region = "us"
}

resource "mondoo_space" "my_space" {
  name   = "Your Slack Space"
  org_id = "your-org-1234567"
}

resource "mondoo_integration_slack" "slack_integration" {
  space_id = mondoo_space.my_space.id
  name     = "Slack Integration"

  slack_token = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
}