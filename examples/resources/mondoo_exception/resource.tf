variable "space_id" {
  type        = string
  description = "The ID of the mondoo space."
}

provider "mondoo" {
  region = "eu"
  space  = var.space_id
}

data "mondoo_assets" "assets_data" {
  space_id = var.space_id
}

locals {
  ssl_asset = [for asset in data.mondoo_assets.assets_data.assets : asset if startswith(asset.name, "https")]
  asset_id  = one(local.ssl_asset).id
}


resource "mondoo_exception" "exception" {
  scope_mrn     = "//assets.api.mondoo.app/spaces/${var.space_id}/assets/${local.asset_id}"
  valid_until   = "2025-12-11"
  justification = "testing"
  action        = "RISK_ACCEPTED"
  check_mrns    = ["//policy.api.mondoo.app/queries/mondoo-tls-security-mitigate-beast"]
}
