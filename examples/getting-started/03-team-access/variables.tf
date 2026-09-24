variable "org_id" {
  description = "ID of the Mondoo organization, for example \"lucid-wing-123456\"."
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

variable "security_team_members" {
  description = "Email addresses of the security team. People who don't have a Mondoo account yet get a pending membership."
  type        = list(string)
  default     = []
}

variable "app_team_members" {
  description = "Email addresses of the application team."
  type        = list(string)
  default     = []
}

variable "security_team_idp_group" {
  description = "Optional: name or ID of a group in your identity provider (from the OIDC groups claim). Members of that group join the security team automatically when they sign in with SSO."
  type        = string
  default     = null
}
