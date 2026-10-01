# Set the SLAs for every space in the organization. While this resource
# exists, spaces use these SLAs and can't change them. Destroying it hands
# each space back its own SLA settings, or the Mondoo defaults.
resource "mondoo_organization_sla" "example" {
  org_id = var.org_id

  critical = {
    days_to_resolve     = 7
    days_before_warning = 5
  }

  high = {
    days_to_resolve     = 30
    days_before_warning = 23
  }

  medium = {
    days_to_resolve     = 60
    days_before_warning = 53
  }

  low = {
    days_to_resolve     = 90
    days_before_warning = 83
  }

  # Optional, shown with their defaults.
  start_date_config = "CVE_DETECTED"
  rating_source     = "RISK"
}
