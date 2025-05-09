variable "foo" {
  description = "The foo variable"
  type        = string
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the GoogleWorkspace integration
resource "mondoo_integration_google_workspace" "example" {
  name = "GoogleWorkspace Integration"
  foo  = var.foo
}
