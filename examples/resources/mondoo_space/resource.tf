variable "org_id" {
  description = "The ID of the organization in which to create the spaces"
  type        = string
}

provider "mondoo" {}

resource "mondoo_space" "my_space" {
  name        = "My New Space"
  description = "A space used to secure my environment"
  # optional id otherwise it will be auto-generated
  # id = "your-space-id"
  org_id = var.org_id
}
