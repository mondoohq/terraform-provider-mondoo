resource "mondoo_organization_platform_policies_opt_out" "example" {
  provider = mondoo.platform_admin
  org_id   = var.org_id
}
