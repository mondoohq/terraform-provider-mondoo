variable "client_secret" {
  description = "The foo variable"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the SentinelOne integration
resource "mondoo_integration_sentinel_one" "example" {
  name    = "SentinelOne Integration"
  host    = "domain.sentinelone.net"
  account = "Your Account"

  credentials = {
    client_secret = var.client_secret
  }
}
