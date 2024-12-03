variable "jira_token" {
  description = "The Jira API Token"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the Jira integration
resource "mondoo_integration_jira" "jira_integration" {
  name  = "My Jira Integration"
  host  = "https://your-instance.atlassian.net"
  email = "jira.owner@email.com"
  # default_project = "MONDOO"
  api_token = var.jira_token

  auto_create = true
  auto_close  = true
}
