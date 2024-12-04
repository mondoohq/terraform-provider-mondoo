variable "spaceId" {
  type = string 
}

provider "mondoo" {
  region = "eu"
  space = var.spaceId
}

data "mondoo_assets" "assets_data" {
  space_id = var.spaceId
}

locals { 
  ssl_asset = [ for asset in data.mondoo_assets.assets_data.assets: asset if startswith(asset.name, "https") ]
  assetId = one(local.ssl_asset).id
}


resource "mondoo_exception" "exception" {
  scope_mrn = "//assets.api.mondoo.app/spaces/${var.spaceId}/assets/${local.assetId}"
  valid_until = "2024-12-12T09:33:46.206Z"
  justification = "testing"
  action ="SNOOZE"
  // check_mrns = ["//policy.api.mondoo.app/queries/mondoo-tls-security-no-weak-block-cipher-modes"]
   check_mrns = [ "//policy.api.mondoo.app/queries/mondoo-tls-security-mitigate-beast" ]
}
