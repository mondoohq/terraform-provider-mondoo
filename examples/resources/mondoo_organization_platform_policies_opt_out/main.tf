terraform {
  required_providers {
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.52"
    }
  }
}

# Only a platform admin can opt an organization out of platform-level policies. Use a
# provider configuration with a platform admin service account for this resource.
provider "mondoo" {
  alias       = "platform_admin"
  credentials = var.platform_admin_credentials
}

variable "platform_admin_credentials" {
  description = "Credentials of a service account with the platform admin role"
  type        = string
  sensitive   = true
}

variable "org_id" {
  description = "The ID of the organization to opt out"
  type        = string
}
