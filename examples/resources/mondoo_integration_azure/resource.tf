terraform {
  required_providers {
    azuread = {
      source  = "hashicorp/azuread"
      version = "2.48.0"
    }
    mondoo = {
      source = "mondoohq/mondoo"
    }
  }
}

provider "azuread" {}

data "azuread_client_config" "current" {}

data "azuread_application" "mondoo-security" {
  display_name = "mondoo-security"
}

provider "mondoo" {
  region = "us"
}

variable "mondoo_org" {
  description = "Mondoo Organization"
  type        = string
}

// Create a new space
resource "mondoo_space" "azure_space" {
  name   = "Azure ${data.azuread_application.mondoo-security.display_name}"
  org_id = var.mondoo_org
}

// Setup the Azure integration
resource "mondoo_integration_azure" "azure_integration" {
  space_id  = mondoo_space.azure_space.id
  name      = "Azure ${data.azuread_application.mondoo-security.display_name}"
  tenant_id = data.azuread_client_config.current.tenant_id
  client_id = data.azuread_application.mondoo-security.client_id
  scan_vms  = true
  # subscription_allow_list= ["ffffffff-ffff-ffff-ffff-ffffffffffff", "ffffffff-ffff-ffff-ffff-ffffffffffff"]
  # subscription_deny_list = ["ffffffff-ffff-ffff-ffff-ffffffffffff", "ffffffff-ffff-ffff-ffff-ffffffffffff"]
  credentials = {
    pem_file = <<EOT
-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCf2kWtE6JkkP6E
cnQx/1oa4GqFs23nJFBQhgn9AThqAyUC1ilLQV9ZKjQj5/6+ljq/i4H/zU5lt2yB
....
qpbiCwjFYHmjWFygtYPhRH4T5TEzu4DXhjr4nn99sF0QFKcYkcTSIm7aZppYG4OS
1fnF+XoTcyFIGcSX/I1ND/4=
-----END PRIVATE KEY-----
EOT
  }
}
