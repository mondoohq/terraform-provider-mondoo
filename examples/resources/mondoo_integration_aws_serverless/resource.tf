variable "mondoo_org" {
  description = "Mondoo Organization"
  type        = string
}

variable "mondoo_token" {
  description = "Mondoo API Token"
  type        = string
}

variable "origin_aws_account" {
  description = "Origin AWS Account"
  type        = string
  default = "764453172858"
}

provider "mondoo" {
  region = "eu"
}

provider "aws" {
  region = "us-east-1"
}

data "aws_region" "current" {}

# Create a new space
resource "mondoo_space" "my_space" {
  name   = "AWS Terraform"
  org_id = var.mondoo_org
}

# Setup the AWS integration
resource "mondoo_integration_aws_serverless" "example" {
  space_id                     = mondoo_space.my_space.id
  name                         = "AWS Integration"
  region                       = data.aws_region.current.name
  # account_ids                  = ["123456789012"]
  is_organization              = true
  console_sign_in_trigger      = true
  instance_state_change_trigger = false

  scan_configuration {
    account_scan       = true
    ec2_scan           = true
    ecr_scan           = false
    ecs_scan           = false
    cron_scanin_hours  = 24
    
    ec2_scan_options {
      ssm                = true
      # instance_ids_filter = ["i-1234567890abcdef0"]
      # regions_filter     = ["us-west-1", "us-west-2"]
      # tags_filter        = {
      #   "Environment" = "Production"
      # }
      ebs_volume_scan    = true
      ebs_scan_options {
        target_instances_per_scanner = 5
        max_asg_instances           = 10
      }
      instance_connect   = false
    }
  }
}

resource "aws_cloudformation_stack" "mondoo_stack" {
  name = "mondoo-stack"
  # region = "us-east-1"
  template_url = "https://s3.amazonaws.com/mondoo.${data.aws_region.current.name}/mondoo-lambda-stackset-cf.json"
  parameters = {
    MondooIntegrationMrn = mondoo_integration_aws_serverless.example.mrn
    MondooToken          = var.mondoo_token
    OriginAwsAccount     = var.origin_aws_account

  }
}