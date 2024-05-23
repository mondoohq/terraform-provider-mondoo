terraform {
  required_providers {
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.4.0"
    }
    aws = {
      source = "hashicorp/aws"
      version = "5.50.0"
    }
  }
}