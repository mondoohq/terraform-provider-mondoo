# Variables
# ----------------------------------------------

variable "tenant_id" {
  description = "The Azure Active Directory Tenant ID"
  type        = string
  default     = "ffffffff-ffff-ffff-ffff-ffffffffffff"
}

variable "primary_subscription" {
  description = "The primary Azure Subscription ID"
  type        = string
  default     = "ffffffff-ffff-ffff-ffff-ffffffffffff"
}

locals {
  mondoo_security_integration_name = "Mondoo Security Integration"
}

# Azure AD with Application and Certificate
# ----------------------------------------------

provider "azuread" {
  tenant_id = var.tenant_id
}

data "azuread_client_config" "current" {}

# Add the required permissions to the application
# User still need to be grant the permissions to the application via the Azure Portal
resource "azuread_application" "mondoo_security" {
  display_name = local.mondoo_security_integration_name

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
      id   = "6e472fd1-ad78-48da-a0f0-97ab2c6b769e"
      type = "Role"
    }

    resource_access {
      id   = "dc5007c0-2d7d-4c42-879c-2dab87571379"
      type = "Role"
    }

    resource_access {
      id   = "b0afded3-3588-46d8-8b3d-9842eff778da"
      type = "Role"
    }

    resource_access {
      id   = "7ab1d382-f21e-4acd-a863-ba3e13f7da61"
      type = "Role"
    }

    resource_access {
      id   = "197ee4e9-b993-4066-898f-d6aecc55125b"
      type = "Role"
    }

    resource_access {
      id   = "9a5d68dd-52b0-4cc2-bd40-abcf44ac3a30"
      type = "Role"
    }

    resource_access {
      id   = "f8f035bb-2cce-47fb-8bf5-7baf3ecbee48"
      type = "Role"
    }

    resource_access {
      id   = "dbb9058a-0e50-45d7-ae91-66909b5d4664"
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

    resource_access {
      id   = "c7fbd983-d9aa-4fa7-84b8-17382c103bc4"
      type = "Role"
    }
  }
}

resource "tls_private_key" "credential" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "tls_self_signed_cert" "credential" {
  private_key_pem = tls_private_key.credential.private_key_pem

  # Certificate expires after 3 months.
  validity_period_hours = 1680

  # Generate a new certificate if Terraform is run within three
  # hours of the certificate's expiration time.
  early_renewal_hours = 3

  # Reasonable set of uses for a server SSL certificate.
  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "data_encipherment",
    "cert_signing",
  ]

  subject {
    common_name = "mondoo"
  }
}

# Attach the certificate to the application
resource "azuread_application_certificate" "mondoo_security_integration" {
  # see https://github.com/hashicorp/terraform-provider-azuread/issues/1227
  application_id = azuread_application.mondoo_security.id
  type           = "AsymmetricX509Cert"
  value          = tls_self_signed_cert.credential.cert_pem
}

# Create a service principal for the application
resource "azuread_service_principal" "mondoo_security" {
  client_id                    = azuread_application.mondoo_security.client_id
  app_role_assignment_required = false
  owners                       = [data.azuread_client_config.current.object_id]
}

# Azure Permissions to Azure AD Application
# ----------------------------------------------

provider "azurerm" {
  tenant_id = var.tenant_id
  features {}
}

data "azurerm_subscription" "primary" {
  subscription_id = var.primary_subscription
}

data "azurerm_subscriptions" "available" {}

# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/role_definition
resource "azurerm_role_definition" "mondoo_security_role" {
  name        = "tf-mondoo-security-role"
  description = "This role includes all permissions for Mondoo Security to assess the security."
  scope       = data.azurerm_subscription.primary.id

  permissions {
    actions = [
      "Microsoft.Authorization/*/read",
      "Microsoft.ResourceHealth/availabilityStatuses/read",
      "Microsoft.Insights/alertRules/*",
      "Microsoft.Resources/deployments/*",
      "Microsoft.Resources/subscriptions/resourceGroups/read",
      "Microsoft.Support/*",
      "Microsoft.Web/listSitesAssignedToHostName/read",
      "Microsoft.Web/serverFarms/read",
      "Microsoft.Web/sites/config/read",
      "Microsoft.Web/sites/config/web/appsettings/read",
      "Microsoft.Web/sites/config/web/connectionstrings/read",
      "Microsoft.Web/sites/config/appsettings/read",
      "Microsoft.web/sites/config/snapshots/read",
      "Microsoft.Web/sites/config/list/action",
      "Microsoft.Web/sites/read",
      "Microsoft.KeyVault/checkNameAvailability/read",
      "Microsoft.KeyVault/deletedVaults/read",
      "Microsoft.KeyVault/locations/*/read",
      "Microsoft.KeyVault/vaults/*/read",
      "Microsoft.KeyVault/operations/read",
      "Microsoft.Compute/virtualMachines/runCommands/read",
      "Microsoft.Compute/virtualMachines/runCommands/write",
      "Microsoft.Compute/virtualMachines/runCommands/delete"
    ]
    not_actions = []
    data_actions = [
      "Microsoft.KeyVault/vaults/*/read",
      "Microsoft.KeyVault/vaults/secrets/readMetadata/action"
    ]
    not_data_actions = []
  }

  assignable_scopes = data.azurerm_subscriptions.available.subscriptions[*].id
}

# add custom role to all subscriptions
resource "azurerm_role_assignment" "mondoo_security" {
  count              = length(data.azurerm_subscriptions.available.subscriptions)
  scope              = data.azurerm_subscriptions.available.subscriptions[count.index].id
  role_definition_id = azurerm_role_definition.mondoo_security_role.role_definition_resource_id
  principal_id       = azuread_service_principal.mondoo_security.object_id
  depends_on = [
    azurerm_role_definition.mondoo_security_role,
  ]
}

# add reader role to all subscriptions
resource "azurerm_role_assignment" "reader" {
  count                = length(data.azurerm_subscriptions.available.subscriptions)
  scope                = data.azurerm_subscriptions.available.subscriptions[count.index].id
  role_definition_name = "Reader"
  principal_id         = azuread_service_principal.mondoo_security.object_id
}

# Configure the Mondoo
# ----------------------------------------------

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the MsDefender integration
resource "mondoo_integration_msdefender" "msdefender_integration" {
  name      = "Azure ${local.mondoo_security_integration_name}"
  tenant_id = var.tenant_id
  client_id = azuread_application.mondoo_security.client_id

  # subscription_allow_list= ["ffffffff-ffff-ffff-ffff-ffffffffffff", "ffffffff-ffff-ffff-ffff-ffffffffffff"]
  # subscription_deny_list = ["ffffffff-ffff-ffff-ffff-ffffffffffff", "ffffffff-ffff-ffff-ffff-ffffffffffff"]
  credentials = {
    pem_file = join("\n", [tls_self_signed_cert.credential.cert_pem, tls_private_key.credential.private_key_pem])
  }
  # wait for the permissions to provisioned
  depends_on = [
    azuread_application.mondoo_security,
    azuread_service_principal.mondoo_security,
    azurerm_role_assignment.mondoo_security,
    azurerm_role_assignment.reader,
  ]
}
