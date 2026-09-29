resource "mondoo_space" "this" {
  org_id      = var.org_id
  name        = "Custom policies and exceptions"
  description = "Created by the Mondoo Terraform custom-policy example."
}

# Upload a policy you wrote. source is a path relative to where you run
# Terraform; path.module makes it work from any directory. When the file's
# contents change, terraform apply uploads the new version.
resource "mondoo_custom_policy" "ssh" {
  space_id  = mondoo_space.this.id
  source    = "${path.module}/policies/ssh-hardening.mql.yaml"
  overwrite = true
}

# Uploading a policy doesn't enable it. Assign your custom policy together
# with public ones. mrns lists every policy in the uploaded file.
resource "mondoo_policy_assignment" "this" {
  scope_mrn = mondoo_space.this.mrn
  policies = concat(
    mondoo_custom_policy.ssh.mrns,
    [
      "//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2022-ms-level-1",
    ],
  )
  state = "enabled"
}

# An exception records that you know about failing checks and why they're
# acceptable, so they stop counting against your score. Here, the CIS benchmark
# expects Microsoft Defender Antivirus, but these servers run a different EDR
# product. valid_until makes the decision expire instead of lasting forever.
resource "mondoo_exception" "defender_replaced_by_edr" {
  scope_mrn     = mondoo_space.this.mrn
  action        = "RISK_ACCEPTED"
  justification = "Microsoft Defender Antivirus is turned off because a third-party EDR agent runs on every server."
  valid_until   = var.exception_valid_until

  check_mrns = [
    "//policy.api.mondoo.app/queries/cis-microsoft-windows-server-2022--18.10.42.5.1",
  ]

  # Create the exception after the policy that contains the checks is assigned.
  depends_on = [mondoo_policy_assignment.this]
}
