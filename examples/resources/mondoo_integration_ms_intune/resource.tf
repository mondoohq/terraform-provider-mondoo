variable "tenant_id" {
  description = "The Azure Active Directory Tenant ID"
  type        = string
}

variable "client_id" {
  description = "The Azure Application (Client) ID"
  type        = string
}

variable "client_secret" {
  description = "The Azure Application Client Secret"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

resource "mondoo_integration_ms_intune" "intune_integration" {
  name      = "Intune Integration"
  tenant_id = var.tenant_id
  client_id = var.client_id
  credentials = {
    client_secret = var.client_secret
  }
}
