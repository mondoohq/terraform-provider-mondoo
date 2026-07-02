variable "gcp_organization_id" {
  description = "The GCP organization ID (or folder ID) to scan."
  type        = string
}

variable "gcp_host_project_id" {
  description = "The GCP project ID where the serverless scanner is deployed."
  type        = string
}

variable "gcp_region" {
  description = "The GCP region where the serverless scanner is deployed."
  type        = string
  default     = "us-central1"
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the GCP serverless integration
resource "mondoo_integration_gcp_serverless" "gcp_serverless" {
  name            = "GCP Serverless Integration"
  scope           = var.gcp_organization_id
  host_project_id = var.gcp_host_project_id
  region          = var.gcp_region

  scan_configuration = {
    # Only scan projects tagged for production. A value of "*" matches any value.
    tags_filter = {
      "env" = "production"
    }
    # Skip sandbox projects regardless of the include filter.
    excluded_tags_filter = {
      "env" = "sandbox"
    }
  }
}
