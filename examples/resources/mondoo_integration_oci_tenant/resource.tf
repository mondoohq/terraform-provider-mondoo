# Variables
# ----------------------------------------------

variable "mondoo_org" {
  description = "The Mondoo Organization ID"
  type        = string
  default     = "your-org-1234567"
}

# Configure the Mondoo
# ----------------------------------------------

provider "mondoo" {
  region = "us"
}

resource "mondoo_space" "my_space" {
  name   = "My Space Name"
  org_id = var.mondoo_org
}

# Setup the OCI integration
resource "mondoo_integration_oci_tenant" "tenant_abc" {
  space_id = mondoo_space.my_space.id
  name     = "tenant ABC"

  tenancy = "ocid1.tenancy.oc1..aaaaaaaavvvvvvvvwwwwwwwwxxxxxx..."
  region  = "us-ashburn-1"
  user    = "ocid1.user.oc1..aaaaaaaabbbbbbbbccccccccddddeeeeee..."

  credentials = {
    fingerprint = "12:34:56:78:9a:bc:de:f1:23:45:67:89:ab:cd:ef:12"
    private_key = <<EOT
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
