provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the Email integration
resource "mondoo_integration_email" "email_integration" {
  name = "My Email Integration"

  recipients = [
    {
      name          = "John Doe"
      email         = "john@example.com"
      is_default    = true
      reference_url = "https://example.com"
    },
    {
      name          = "Alice Doe"
      email         = "alice@example.com"
      is_default    = false
      reference_url = "https://example.com"
    }
  ]

  auto_create = true
  auto_close  = true
}
