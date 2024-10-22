variable "org_id" {
  description = "The organization id to create the spaces in"
  type        = string
}

provider "mondoo" {}

resource "mondoo_space" "my_space" {
  name = "My Space New"
  # optional id otherwise it will be auto-generated
  # id = "your-space-id"
  org_id = var.org_id
}
