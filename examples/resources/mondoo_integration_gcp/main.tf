terraform {
  required_providers {
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.19"
    }
    google = {
      source  = "hashicorp/google"
      version = ">= 5.26.0"
    }
  }
}