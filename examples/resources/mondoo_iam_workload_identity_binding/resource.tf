provider "mondoo" {
  space = "hungry-poet-123456"
}

resource "mondoo_iam_workload_identity_binding" "example" {
  name       = "Example Binding"
  issuer_uri = "https://accounts.google.com"
  subject    = "foo"
}
