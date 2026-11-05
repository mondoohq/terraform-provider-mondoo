variable "jira_token" {
  description = "The Jira API Token"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Set up the Jira integration
resource "mondoo_integration_jira" "jira_integration" {
  name  = "My Jira Integration"
  host  = "https://your-instance.atlassian.net"
  email = "jira.owner@email.com"
  # default_project = "MONDOO"

  auto_create = true
  auto_close  = true

  credentials = {
    token = var.jira_token
  }
}

# Or set it up once for the organization: every space of the organization can
# file tickets through it. An organization integration cannot set auto_create.
resource "mondoo_integration_jira" "org_jira_integration" {
  org_id = "your-org-1234567"
  name   = "Organization Jira"
  host   = "https://your-instance.atlassian.net"
  email  = "jira.owner@email.com"

  auto_close = true

  credentials = {
    token = var.jira_token
  }
}
