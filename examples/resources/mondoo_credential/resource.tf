variable "gh_token" {
  description = "GitHub personal access token"
  type        = string
  sensitive   = true
}

variable "slack_bot_token" {
  description = "Slack bot token"
  type        = string
  sensitive   = true
}

# A space-scoped credential.
resource "mondoo_credential" "github_scanner" {
  space_id = "hungry-poet-123456"
  name     = "github-scanner"

  github_pat = {
    token    = var.gh_token
    base_url = "https://github.acme.com"
  }
}

# An organization-scoped credential. Changing the scope of an existing
# credential replaces it, and a credential in use cannot be deleted — so if an
# integration references it, invert the replacement order.
resource "mondoo_credential" "slack_bot" {
  scope_mrn = "//captain.api.mondoo.app/organizations/dazzling-hopper-123456"
  name      = "slack-bot"

  slack = {
    bot_token = var.slack_bot_token
  }

  lifecycle {
    create_before_destroy = true
  }
}

# An integration consuming a credential. The dependency makes Terraform destroy
# the integration before the credential, which is what keeps `terraform destroy`
# working: deleting a credential an integration still references is refused.
resource "mondoo_integration_github" "scanner" {
  space_id       = "hungry-poet-123456"
  name           = "acme-org"
  owner          = "acme"
  credential_mrn = mondoo_credential.github_scanner.mrn
}
