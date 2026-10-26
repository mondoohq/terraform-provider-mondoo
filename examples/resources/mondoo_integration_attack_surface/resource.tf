provider "mondoo" {
  space = "hungry-poet-123456"
}

# Scan a domain and its subdomains every day, with the default scans
resource "mondoo_integration_attack_surface" "example" {
  domains = ["example.com"]
}

# Scan two domains weekly, with subdomain takeover and cloud discovery turned on
resource "mondoo_integration_attack_surface" "weekly" {
  name               = "Example domains, weekly"
  domains            = ["example.org", "example.net"]
  schedule           = "168h"
  subdomain_takeover = true
  cloud_discovery    = true
}
