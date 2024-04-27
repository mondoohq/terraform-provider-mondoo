# Variables
# ----------------------------------------------

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

# Configure the Mondoo
# ----------------------------------------------

provider "mondoo" {
  region = "us"
}

resource "mondoo_space" "my_space" {
  count  = length(var.space_names)
  name   = var.space_names[count.index]
  org_id = var.org_id
}

resource "mondoo_registration_token" "token" {
  description   = "Get a mondoo registration token"
  count         = length(var.space_names)
  space_id      = mondoo_space.my_space[count.index].id
  no_exipration = true
  # define optional expiration
  # expires_in = "1h"
  depends_on = [
    mondoo_space.my_space
  ]
}

output "space_registration_token" {
  description = "The list of space registration tokens for the specified spaces"
  value = [
    for count, space in mondoo_space.my_space :
    {
      space-name : space.name,
      space-id : space.id,
      token : mondoo_registration_token.token[count].result
    }
  ]
  sensitive = true
}
