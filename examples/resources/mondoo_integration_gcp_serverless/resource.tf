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

# The numeric unique ID of the GCP service account the deployed scanner runs
# as. Used as the WIF binding subject; required when use_wif is true.
variable "gcp_service_account_id" {
  description = "Numeric unique ID of the scanner's GCP service account."
  type        = string
  default     = ""
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

  # Authenticate the deployed scanner back to the platform via GCP Workload
  # Identity Federation. When enabled, a WIF binding is minted at create time
  # and its config is returned as `wif_config` below.
  use_wif            = true
  service_account_id = var.gcp_service_account_id

  scan_configuration = {
    # Only scan projects tagged for production. A value of "*" matches any value.
    tags_filter = {
      "env" = "production"
    }
    # Skip sandbox projects regardless of the include filter.
    excluded_tags_filter = {
      "env" = "sandbox"
    }
    # Propagate GCP project tags onto all discovered assets.
    propagate_project_tags = true
  }
}

# The base64-encoded WIF external account configuration for the deployed
# scanner. Pass it to the serverless stack's Terraform deployment.
output "gcp_serverless_wif_config" {
  description = "Base64-encoded WIF external account configuration for the deployed GCP serverless scanner."
  value       = mondoo_integration_gcp_serverless.gcp_serverless.wif_config
}
