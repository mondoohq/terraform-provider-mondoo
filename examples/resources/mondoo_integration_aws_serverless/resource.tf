variable "mondoo_org" {
  description = "Mondoo Organization"
  type        = string
}

variable "origin_aws_account" {
  description = "Origin AWS Account"
  type        = string
  default     = "764453172858"
}

variable "aws_region" {
  description = "AWS Region"
  type        = string
  default     = "us-east-1"
}

variable "aws_account_id" {
  description = "value of the AWS account ID"
  type        = string
}

provider "mondoo" {
  region = "us"
}

provider "aws" {
  region = var.aws_region
}

data "aws_region" "current" {}

# Create a new space
resource "mondoo_space" "my_space" {
  name   = "AWS Terraform"
  org_id = var.mondoo_org
}

# Setup the AWS integration
resource "mondoo_integration_aws_serverless" "aws_serverless" {
  space_id                      = mondoo_space.my_space.id
  name                          = "AWS Integration"
  region                        = data.aws_region.current.name
  is_organization               = false
  console_sign_in_trigger       = true
  instance_state_change_trigger = true
  account_ids                   = [var.aws_account_id]
  scan_configuration = {
    ec2_scan          = true
    ecr_scan          = false
    ecs_scan          = false
    cron_scanin_hours = 24
    ec2_scan_options = {
      ssm             = true
      ebs_volume_scan = true
      ebs_scan_options = {
        target_instances_per_scanner = 5
        max_asg_instances            = 10
      }
      instance_connect = false
    }
  }
}

# for single account deploys
resource "aws_cloudformation_stack" "mondoo_stack" {
  name         = "mondoo-stack"
  template_url = "https://s3.amazonaws.com/mondoo.${data.aws_region.current.name}/mondoo-lambda-stackset-cf.json"
  capabilities = ["CAPABILITY_NAMED_IAM"]
  parameters = {
    MondooIntegrationMrn = mondoo_integration_aws_serverless.aws_serverless.mrn
    MondooToken          = mondoo_integration_aws_serverless.aws_serverless.token
    OriginAwsAccount     = var.origin_aws_account
  }
}

# for organisation wide deploys use aws_cloudformation_stack_set and aws_cloudformation_stack_set_instance instaed of aws_cloudformation_stack
# https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudformation_stack_set
# https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudformation_stack_set_instance
