variable "foo" {
  description = "The foo variable"
  type        = string
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the AzureDevops integration
resource "mondoo_integration_azure_devops" "example" {
  name = "AzureDevops Integration"
  foo  = var.foo
}
