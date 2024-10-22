terraform {
  required_providers {
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.19"
    }
    aws = {
      source  = "hashicorp/aws"
      version = "5.50.0"
    }
  }
}