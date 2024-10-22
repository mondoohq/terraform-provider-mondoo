variable "aws_access_key" {
  description = "AWS access key"
  type        = string
  sensitive   = true
}

variable "aws_secret_key" {
  description = "AWS secret key"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the AWS integration
resource "mondoo_integration_aws" "name" {
  name = "AWS Integration"

  credentials = {
    key = {
      access_key = var.aws_access_key
      secret_key = var.aws_secret_key
    }
  }
}
