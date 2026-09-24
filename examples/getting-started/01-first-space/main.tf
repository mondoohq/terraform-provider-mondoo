# A space holds assets (machines, cloud accounts, repositories, ...) and the
# policies that Mondoo evaluates against them.
resource "mondoo_space" "this" {
  org_id      = var.org_id
  name        = var.space_name
  description = "Created by the Mondoo Terraform getting-started example."
}

# Enable security policies. Every asset scanned into the space is checked
# against the policies that apply to its platform, so it's fine to enable
# Linux, macOS, and Windows policies side by side.
resource "mondoo_policy_assignment" "security" {
  scope_mrn = mondoo_space.this.mrn
  policies  = var.policies
  state     = "enabled"
}

# Compliance frameworks map policy results to controls, such as CIS Controls
# or ISO 27001, and show your progress toward each one.
resource "mondoo_framework_assignment" "compliance" {
  space_id      = mondoo_space.this.id
  framework_mrn = var.frameworks
  enabled       = true
}

# A registration token lets cnspec register a machine in this space. This one
# expires after 24 hours, which is enough to try it out. expires_in takes a
# duration in hours, minutes, or seconds, such as "24h" or "720h" (30 days).
resource "mondoo_registration_token" "scan" {
  space_id    = mondoo_space.this.id
  description = "Getting-started token for ${var.space_name}"
  expires_in  = "24h"
}
