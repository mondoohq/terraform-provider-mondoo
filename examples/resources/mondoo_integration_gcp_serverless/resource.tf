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

# Set up the GCP serverless integration
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
    project_tags = {
      "env" = "production"
    }
    # Within those projects, only scan resources tagged for scanning.
    tags_filter = {
      "scan" = "true"
    }
    # Skip resources tagged as sandbox regardless of the include filter.
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

# Inputs for the serverless scanner's Terraform deployment: the orchestrator
# image this platform version expects (`image`) and the service account the
# platform sends on-demand commands as (`mondoo_actor_service_account`).
output "gcp_serverless_orchestrator_image" {
  description = "Orchestrator container image the platform expects for the deployed GCP serverless scanner."
  value       = mondoo_integration_gcp_serverless.gcp_serverless.orchestrator_image
}

output "gcp_serverless_actor_service_account" {
  description = "Service account the platform sends on-demand commands to the deployed GCP serverless scanner as."
  value       = mondoo_integration_gcp_serverless.gcp_serverless.actor_service_account
}

variable "mondoo_organization_mrn" {
  description = "The Mondoo organization MRN to create the cross-org integration under."
  type        = string
  default     = ""
}

# Cross-org integration: created at the Mondoo *organization* scope (via an
# organization scope_mrn) so its assets can land in spaces across every org.
# cross_org requires use_wif and an organization-scoped scope_mrn.
resource "mondoo_integration_gcp_serverless" "gcp_serverless_crossorg" {
  name      = "GCP Serverless Cross-Org Integration"
  scope_mrn = var.mondoo_organization_mrn

  host_project_id = var.gcp_host_project_id
  region          = var.gcp_region

  cross_org          = true
  use_wif            = true
  service_account_id = var.gcp_service_account_id
}
