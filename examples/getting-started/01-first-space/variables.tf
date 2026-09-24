variable "org_id" {
  description = "ID of the Mondoo organization to create the space in, for example \"lucid-wing-123456\"."
  type        = string
}

variable "region" {
  description = "Region of your Mondoo organization: \"us\" or \"eu\"."
  type        = string
  default     = "us"

  validation {
    condition     = contains(["us", "eu"], var.region)
    error_message = "The region must be \"us\" or \"eu\"."
  }
}

variable "space_name" {
  description = "Display name of the new space."
  type        = string
  default     = "Getting started with Terraform"
}

variable "policies" {
  description = "MRNs of the policies to enable in the space. To see which policies are available, use the mondoo_policies data source."
  type        = list(string)
  default = [
    "//policy.api.mondoo.app/policies/mondoo-linux-security",
    "//policy.api.mondoo.app/policies/mondoo-macos-security",
    "//policy.api.mondoo.app/policies/mondoo-windows-security",
  ]
}

variable "frameworks" {
  description = "MRNs of the compliance frameworks to enable in the space."
  type        = list(string)
  default = [
    "//policy.api.mondoo.app/frameworks/cis-controls-8",
  ]
}
