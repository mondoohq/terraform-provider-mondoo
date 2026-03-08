provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the webhook ticketing integration
resource "mondoo_integration_webhook" "webhook_integration" {
  name = "My Webhook Integration"
  url  = "https://example.com/webhook"

  auto_create = true
  auto_close  = true
}
