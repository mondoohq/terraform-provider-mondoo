variable "client_secret" {
  description = "The SentinelOne client secret"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Set up the SentinelOne integration
resource "mondoo_integration_sentinel_one" "example" {
  name    = "SentinelOne Integration"
  host    = "https://domain.sentinelone.net"
  account = "Your Account"

  credentials = {
    client_secret = var.client_secret
  }
}
