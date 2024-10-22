variable "shodan_token" {
  description = "The Shodan Token"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the Shodan integration
resource "mondoo_integration_shodan" "shodan_integration" {
  name    = "Shodan Integration"
  targets = ["8.8.8.8", "mondoo.com", "63.192.236.0/24"]

  credentials = {
    token = var.shodan_token
  }
}
