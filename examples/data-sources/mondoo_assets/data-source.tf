provider "mondoo" {}

data "mondoo_assets" "assets_data" {
  space_id = "my-space-1234567"
}

output "asset_mrns" {
  description = "MRNs of the assets"
  value       = [for asset in data.mondoo_assets.assets_data.assets : asset.mrn]
}

output "asset_names" {
  description = "Names of the assets"
  value       = [for asset in data.mondoo_assets.assets_data.assets : asset.name]
}
