# Workload Identity Federation (keyless) example
# -----------------------------------------------
#
# This example wires up a Mondoo MS365 integration using Workload Identity
# Federation (WIF). No certificate or client secret is stored — Mondoo
# authenticates as a federated workload.
#
# DAG: azuread_application → mondoo_integration_ms365(use_wif=true) →
#      azuread_application_federated_identity_credential
#
# Usage:
#   terraform init
#   terraform apply -var tenant_id=<your-tenant-id>

# Variables
# ----------------------------------------------

variable "wif_tenant_id" {
  description = "The Azure Active Directory Tenant ID"
  type        = string
  default     = "ffffffff-ffff-ffff-ffff-ffffffffffff"
}

locals {
  mondoo_ms365_wif_integration_name = "Mondoo Security Integration (WIF)"
}

# Azure AD Application
# ----------------------------------------------

provider "azuread" {
  tenant_id = var.wif_tenant_id
}

data "azuread_client_config" "wif_current" {}

# Add the required permissions to the application.
# You still need to grant admin consent for these permissions in the Azure Portal.
resource "azuread_application" "mondoo_ms365_wif" {
  display_name = "Ms365 ${local.mondoo_ms365_wif_integration_name}"

  required_resource_access {
    resource_app_id = "00000003-0000-0000-c000-000000000000" # Microsoft Graph

    resource_access {
      id   = "246dd0d5-5bd0-4def-940b-0421030a5b68"
      type = "Role"
    }

    resource_access {
      id   = "e321f0bb-e7f7-481e-bb28-e3b0b32d4bd0"
      type = "Role"
    }

    resource_access {
      id   = "5e0edab9-c148-49d0-b423-ac253e121825"
      type = "Role"
    }

    resource_access {
      id   = "bf394140-e372-4bf9-a898-299cfc7564e5"
      type = "Role"
    }

    resource_access {
      id   = "dc377aa6-52d8-4e23-b271-2a7ae04cedf3"
      type = "Role"
    }

    resource_access {
      id   = "9e640839-a198-48fb-8b9a-013fd6f6cbcd"
      type = "Role"
    }

    resource_access {
      id   = "37730810-e9ba-4e46-b07e-8ca78d182097"
      type = "Role"
    }
  }

  required_resource_access {
    resource_app_id = "00000003-0000-0ff1-ce00-000000000000" # Office 365 Exchange Online

    resource_access {
      id   = "678536fe-1083-478a-9c59-b99265e6b0d3"
      type = "Role"
    }
  }

  required_resource_access {
    resource_app_id = "00000002-0000-0ff1-ce00-000000000000" # Office 365 Exchange Online (legacy)

    resource_access {
      id   = "dc50a0fb-09a3-484d-be87-e023b12c6440"
      type = "Role"
    }
  }
}

# Create a service principal for the application
resource "azuread_service_principal" "mondoo_ms365_wif" {
  client_id                    = azuread_application.mondoo_ms365_wif.client_id
  app_role_assignment_required = false
  owners                       = [data.azuread_client_config.wif_current.object_id]
}

# Directory permissions
# ----------------------------------------------

resource "azuread_directory_role" "wif_global_reader" {
  display_name = "Global Reader"
}

resource "azuread_directory_role_assignment" "wif_global_reader" {
  role_id             = azuread_directory_role.wif_global_reader.template_id
  principal_object_id = azuread_service_principal.mondoo_ms365_wif.object_id
}

# Mondoo Integration (keyless / WIF)
# ----------------------------------------------

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Step 1: Create the Mondoo integration with use_wif=true. No certificate is
# generated or stored. Mondoo returns wif_subject and wif_issuer_url, which are
# used in step 2.
resource "mondoo_integration_ms365" "wif" {
  name      = "Ms365 ${local.mondoo_ms365_wif_integration_name}"
  tenant_id = var.wif_tenant_id
  client_id = azuread_application.mondoo_ms365_wif.client_id
  use_wif   = true

  # wait for the permissions to be provisioned
  depends_on = [
    azuread_application.mondoo_ms365_wif,
    azuread_service_principal.mondoo_ms365_wif,
    azuread_directory_role_assignment.wif_global_reader,
  ]
}

# Step 2: Wire up the federated identity credential using the subject/issuer
# that Mondoo emits. This allows Mondoo to authenticate without a certificate.
resource "azuread_application_federated_identity_credential" "mondoo_ms365" {
  application_id = azuread_application.mondoo_ms365_wif.id
  display_name   = "mondoo"
  issuer         = mondoo_integration_ms365.wif.wif_issuer_url
  subject        = mondoo_integration_ms365.wif.wif_subject
  audiences      = ["api://AzureADTokenExchange"]
}

# Outputs
# ----------------------------------------------

output "ms365_wif_integration_mrn" {
  value       = mondoo_integration_ms365.wif.mrn
  description = "MRN of the Mondoo MS365 integration"
}

output "ms365_wif_subject" {
  value       = mondoo_integration_ms365.wif.wif_subject
  description = "WIF subject configured on the federated identity credential"
}

output "ms365_wif_issuer_url" {
  value       = mondoo_integration_ms365.wif.wif_issuer_url
  description = "WIF issuer URL configured on the federated identity credential"
}
