variable "foo" {
  description = "The foo variable"
  type        = string
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the Okta integration
resource "mondoo_integration_okta" "example" {
  name = "Okta Integration"
  foo  = var.foo
}
