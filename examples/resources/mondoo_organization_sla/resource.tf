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

  # Optional. A rule gives the assets that carry one of its tags their own
  # SLAs. An asset no rule matches uses the four SLAs above.
  rules = [
    {
      # Internet-facing assets: the attack surface set on the asset, else the
      # one Mondoo detects.
      name = "Internet-facing"
      tags = [{ key = "mondoo.com/attack-surface", value = "internet" }]

      critical = {
        days_to_resolve     = 1
        days_before_warning = 0
      }
      high = {
        days_to_resolve     = 5
        days_before_warning = 2
      }
      medium = {
        days_to_resolve     = 30
        days_before_warning = 7
      }
      low = {
        days_to_resolve     = 60
        days_before_warning = 14
      }
    },
    {
      # Any label or annotation works as a tag.
      name = "Production"
      tags = [{ key = "env", value = "production" }]

      critical = {
        days_to_resolve     = 3
        days_before_warning = 1
      }
      high = {
        days_to_resolve     = 14
        days_before_warning = 7
      }
      medium = {
        days_to_resolve     = 45
        days_before_warning = 30
      }
      low = {
        days_to_resolve     = 90
        days_before_warning = 60
      }
    },
  ]
}
