terraform {
  required_providers {
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.4.0"
    }
    google = {
      source  = "hashicorp/google"
      version = ">= 5.26.0"
    }
  }
}