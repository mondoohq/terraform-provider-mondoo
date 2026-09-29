variable "org_id" {
  description = "ID of the Mondoo organization to create the spaces in, for example \"lucid-wing-123456\"."
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

variable "common_policies" {
  description = "MRNs of the policies to enable in every environment."
  type        = list(string)
  default = [
    "//policy.api.mondoo.app/policies/mondoo-linux-security",
    "//policy.api.mondoo.app/policies/mondoo-windows-security",
  ]
}

variable "environments" {
  description = <<-EOT
    Environments to create a space for, keyed by a short name. Each environment can enable extra policies and choose
    how long to keep assets that stop reporting. require_exception_approval makes exceptions in that space wait for a
    second person to approve them.
  EOT
  type = map(object({
    description                = string
    extra_policies             = optional(list(string), [])
    remove_stale_assets_after  = optional(number, 30)
    require_exception_approval = optional(bool, false)
  }))
  default = {
    production = {
      description                = "Production workloads."
      extra_policies             = ["//policy.api.mondoo.app/policies/mondoo-edr-policy"]
      remove_stale_assets_after  = 90
      require_exception_approval = true
    }
    staging = {
      description = "Pre-release testing."
    }
    development = {
      description               = "Developer sandboxes. Assets come and go quickly."
      remove_stale_assets_after = 7
    }
  }
}
