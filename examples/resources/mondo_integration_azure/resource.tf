terraform {
  required_providers {
    mondoo = {
      source = "mondoohq/mondoo"
    }
  }
}

provider "mondoo" {
  region = "us"
}

// Create a new space
resource "mondoo_space" "azure_space" {
  name   = "Azure Integration w Terraform"
  org_id = "your-org-1234567"
}

// Setup the Azure integration
resource "mondoo_integration_azure" "azure_integration" {
  space_id         = mondoo_space.azure_space.id
  name             = "Azure Integration w Terraform"
  tenant_id        = "ffffffff-ffff-ffff-ffff-ffffffffffff"
  client_id        = "ffffffff-ffff-ffff-ffff-ffffffffffff"
  credentials      = {
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
