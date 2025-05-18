terraform {
  required_providers {
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.21"
    }
  }
}

variable "org_id" {
  description = "The ID of the organization in which to create the spaces"
  type        = string
}

provider "mondoo" {}

data "mondoo_organization" "org" {
  id = var.org_id
}

resource "mondoo_space" "my_space" {
  name   = "My New Space"
  org_id = data.mondoo_organization.org.id
}

# Assign policies to the space

resource "mondoo_policy_assignment" "cis_policy_assignment_enabled" {
  space_id = mondoo_space.my_space.id

  policies = [
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-10-l1-ce",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-10-l1-bl",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-11-l1-ce",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-11-l1-bl",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2016-dc-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2016-ms-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2019-dc-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2019-ms-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2022-dc-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2022-ms-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-azure-windows-server-2019-dc-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-azure-windows-server-2019-ms-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-azure-windows-server-2022-dc-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-azure-windows-server-2022-ms-level-1",
    "//policy.api.mondoo.app/policies/mondoo-edr-policy",
  ]

  state = "enabled"
}

# Set exceptions for Windows policies in the space
resource "mondoo_exception" "windows_defender_exception" {
  scope_mrn     = mondoo_space.my_space.mrn
  justification = "Windows Defender is disabled. Other EDR is used/configured instead."
  action        = "RISK_ACCEPTED"
  check_mrns = [
    "//policy.api.mondoo.app/queries/cis-microsoft-windows-10--18.10.42.5.1",
    "//policy.api.mondoo.app/queries/cis-microsoft-windows-11--18.10.42.5.1",
    "//policy.api.mondoo.app/queries/cis-microsoft-windows-server-2016--18.10.42.5.1",
    "//policy.api.mondoo.app/queries/cis-microsoft-windows-server-2019--18.10.42.5.1",
    "//policy.api.mondoo.app/queries/cis-microsoft-windows-server-2022--18.10.42.5.1",
  ]
  depends_on = [
    mondoo_policy_assignment.cis_policy_assignment_enabled
  ]
}