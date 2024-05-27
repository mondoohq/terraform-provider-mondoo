variable "mondoo_org" {
  description = "Mondoo Organization"
  type        = string
}

variable "origin_aws_account" {
  description = "Origin AWS Account"
  type        = string
  default = "764453172858"
}

provider "mondoo" {
  region = "us"
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
  space_id = mondoo_space.my_space.id
  name     = "AWS Integration"
  region   = data.aws_region.current.name
  is_organization = true
  console_sign_in_trigger = true
  instance_state_change_trigger = true
  # account_ids = ["123456789012"]
  scan_configuration = {
    ec2_scan     = true
    ecr_scan     = false
    ecs_scan     = false
    cron_scanin_hours = 24
    ec2_scan_options = {
      ssm = true
      ebs_volume_scan = true
      ebs_scan_options = {
        target_instances_per_scanner = 5
        max_asg_instances = 10
      }
      instance_connect = false
    }
  }
}

resource "aws_cloudformation_stack" "mondoo_stack" {
  name = "mondoo-stack"
  template_url = "https://s3.amazonaws.com/mondoo.${data.aws_region.current.name}/mondoo-lambda-stackset-cf.json"
  capabilities = ["CAPABILITY_NAMED_IAM"]
  parameters = {
    MondooIntegrationMrn = mondoo_integration_aws_serverless.example.mrn
    MondooToken          = mondoo_integration_aws_serverless.example.token
    OriginAwsAccount     = var.origin_aws_account
  }
}

# https://registry.terraform.io/providers/hashicorp/aws/latest/docs/resources/cloudformation_stack_set
# data "aws_iam_policy_document" "AWSCloudFormationStackSetAdministrationRole_assume_role_policy" {
#   statement {
#     actions = ["sts:AssumeRole"]
#     effect  = "Allow"

#     principals {
#       identifiers = ["cloudformation.amazonaws.com"]
#       type        = "Service"
#     }
#   }
# }

# resource "aws_iam_role" "AWSCloudFormationStackSetAdministrationRole" {
#   assume_role_policy = data.aws_iam_policy_document.AWSCloudFormationStackSetAdministrationRole_assume_role_policy.json
#   name               = "AWSCloudFormationStackSetAdministrationRole"
# }

# resource "aws_cloudformation_stack_set" "example" {
#   administration_role_arn = aws_iam_role.AWSCloudFormationStackSetAdministrationRole.arn
#   name                    = "example"

#   parameters = {
#     VPCCidr = "10.0.0.0/16"
#   }

#   template_body = jsonencode({
#     Parameters = {
#       VPCCidr = {
#         Type        = "String"
#         Default     = "10.0.0.0/16"
#         Description = "Enter the CIDR block for the VPC. Default is 10.0.0.0/16."
#       }
#     }
#     Resources = {
#       myVpc = {
#         Type = "AWS::EC2::VPC"
#         Properties = {
#           CidrBlock = {
#             Ref = "VPCCidr"
#           }
#           Tags = [
#             {
#               Key   = "Name"
#               Value = "Primary_CF_VPC"
#             }
#           ]
#         }
#       }
#     }
#   })
# }

# data "aws_iam_policy_document" "AWSCloudFormationStackSetAdministrationRole_ExecutionPolicy" {
#   statement {
#     actions   = ["sts:AssumeRole"]
#     effect    = "Allow"
#     resources = ["arn:aws:iam::*:role/${aws_cloudformation_stack_set.example.execution_role_name}"]
#   }
# }

# resource "aws_iam_role_policy" "AWSCloudFormationStackSetAdministrationRole_ExecutionPolicy" {
#   name   = "ExecutionPolicy"
#   policy = data.aws_iam_policy_document.AWSCloudFormationStackSetAdministrationRole_ExecutionPolicy.json
#   role   = aws_iam_role.AWSCloudFormationStackSetAdministrationRole.name
# }
