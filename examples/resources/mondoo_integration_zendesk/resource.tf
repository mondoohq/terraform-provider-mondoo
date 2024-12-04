variable "zendesk_token" {
  description = "The GitHub Token"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the zendesk integration
resource "mondoo_integration_zendesk" "zendesk_integration" {
  name      = "My Zendesk Integration"
  subdomain = "your-subdomain"
  email     = "zendeskowner@email.com"
  api_token = var.zendesk_token

  custom_fields = [
    {
      id    = "123456"
      value = "custom_value_1"
    },
    {
      id    = "123457"
      value = "custom_value_2"
    }
  ]

  auto_create = true
  auto_close  = true
}