variable "space_names" {
  description = "Create Spaces with these names"
  type        = list(string)
  default     = []
}

variable "org_id" {
  description = "The organization id to create the spaces in"
  type        = string
  default     = ""
}