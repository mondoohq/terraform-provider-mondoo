variable "action1_client_id" {
  description = "Action1 API client ID"
  type        = string
}

variable "action1_client_secret" {
  description = "Action1 API client secret"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Scan the endpoints of an Action1 organization in the EU region.
# The provider stores the API client as a credential in the space.
resource "mondoo_integration_action1" "example" {
  name            = "Action1 endpoints"
  region          = "eu"
  organization_id = "e5a8b1c2-0000-4000-8000-000000000000"
  client_id       = var.action1_client_id
  client_secret   = var.action1_client_secret
}

# Or use an Action1 credential that already exists in the space
resource "mondoo_integration_action1" "existing_credential" {
  name            = "Action1 endpoints (North America)"
  region          = "na"
  organization_id = "0f3c9d7e-0000-4000-8000-000000000000"
  credential_mrn  = "//captain.api.mondoo.app/spaces/hungry-poet-123456/credentials/2Abd08lk860"
}
