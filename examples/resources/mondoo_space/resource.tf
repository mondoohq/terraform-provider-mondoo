provider "mondoo" {
  region = "us"
}

resource "mondoo_space" "my_space" {
  name = "My Space New"
  # optional id otherwise it will be auto-generated
  # id = "your-space-id"
  org_id = "your-org-1234567"
}

