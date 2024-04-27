terraform {
  required_providers {
    azuread = {
      source  = "hashicorp/azuread"
      version = ">= 2.48.0"
    }
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.4.0"
    }
  }
}
