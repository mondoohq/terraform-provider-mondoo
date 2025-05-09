
variable "organization" {
  description = "The Okta Organization"
  type        = string
}
variable "token" {
  description = "The Okta Token"
  type        = string
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the Okta integration
resource "mondoo_integration_okta" "example" {
  name         = "Okta Integration"
  foo          = var.foo
  organization = var.organization
  token        = var.token
}
