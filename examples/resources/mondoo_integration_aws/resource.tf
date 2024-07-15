variable "mondoo_org" {
  description = "Mondoo Organization"
  type        = string
}

variable "aws_access_key" {
  description = "AWS access key"
  type        = string
}

variable "aws_secret_key" {
  description = "AWS secret key"
  type        = string
}

provider "mondoo" {}

# Create a new space
resource "mondoo_space" "my_space" {
  name   = "AWS Terraform"
  org_id = var.mondoo_org
}

# Setup the AWS integration
resource "mondoo_integration_aws" "name" {
  space_id = mondoo_space.my_space.id
  name     = "AWS Integration"

  credentials = {
    key = {
      access_key = var.aws_access_key
      secret_key = var.aws_secret_key
    }
  }
}
