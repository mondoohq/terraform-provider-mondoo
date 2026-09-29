provider "mondoo" {
  space = "hungry-poet-123456"
}

# Query packs collect information from assets without scoring it, for example
# an inventory of installed packages or data for an incident investigation.
# Once assigned, the query pack runs when assets in the space are scanned.
resource "mondoo_querypack_assignment" "incident_response" {
  querypacks = [
    "//policy.api.mondoo.app/policies/mondoo-incident-response-aws",
  ]
  state = "enabled"
}
