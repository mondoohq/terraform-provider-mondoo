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

variable "exception_valid_until" {
  description = "Date the Windows Defender exception expires, as YYYY-MM-DD. After it, the checks count against your score again, so pick a date to revisit the decision."
  type        = string

  validation {
    condition     = can(regex("^\\d{4}-\\d{2}-\\d{2}$", var.exception_valid_until))
    error_message = "Use the format YYYY-MM-DD, for example \"2027-06-30\"."
  }
}
