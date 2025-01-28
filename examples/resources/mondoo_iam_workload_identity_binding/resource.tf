provider "mondoo" {
  space = "hungry-poet-123456"
}

resource "mondoo_iam_workload_identity_binding" "example" {
  name       = "Github binding example"
  issuer_uri = "https://token.actions.githubusercontent.com"
  subject    = "repo:mondoohq/server:ref:refs/heads/main"
  expiration = 3600
}
