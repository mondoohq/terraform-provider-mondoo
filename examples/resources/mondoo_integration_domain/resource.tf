provider "mondoo" {
  space = "hungry-poet-123456"
}

# Set up the Domain integration
resource "mondoo_integration_domain" "domain_integration" {
  host  = "mondoo.com"
  https = true
  http  = false
}
