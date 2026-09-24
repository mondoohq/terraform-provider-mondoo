variable "aws_region" {
  description = "AWS Region"
  type        = string
  default     = "us-east-1"
}

variable "aws_account_id" {
  description = "The AWS account ID"
  type        = string
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

provider "aws" {
  region = var.aws_region
}

data "aws_region" "current" {}

# Set up the AWS integration
resource "mondoo_integration_aws_serverless" "aws_serverless" {
  name                          = "AWS Integration"
  region                        = data.aws_region.current.region
  is_organization               = false
  console_sign_in_trigger       = true
  instance_state_change_trigger = true
  account_ids                   = [var.aws_account_id]
  scan_configuration = {
    ec2_scan           = true
    ecr_scan           = false
    ecs_scan           = false
    cron_scan_in_hours = 24
    ec2_scan_options = {
      ssm              = true
      ebs_volume_scan  = true
      instance_connect = false
      exclude_tags_filter = {
        "Created By" = "Mondoo"
        "Env"        = "dev"
      }
    }
  }
}

# for single account deploys
resource "aws_cloudformation_stack" "mondoo_stack" {
  name         = "mondoo-stack"
  template_url = mondoo_integration_aws_serverless.aws_serverless.cloud_formation_template_url
  capabilities = ["CAPABILITY_NAMED_IAM"]
  parameters = {
    MondooIntegrationMrn = mondoo_integration_aws_serverless.aws_serverless.mrn
    MondooToken          = mondoo_integration_aws_serverless.aws_serverless.token
    MondooSourceBucket   = mondoo_integration_aws_serverless.aws_serverless.source_bucket
  }
}

# for organization-wide deployments use aws_cloudformation_stack_set and aws_cloudformation_stack_set_instance instead of aws_cloudformation_stack
# set is_organization = true (and leave account_ids unset) so the token does not expire; with false it expires after 30 minutes,
# and accounts that join a targeted OU later fail to register
# https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudformation_stack_set
# https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudformation_stack_set_instance
