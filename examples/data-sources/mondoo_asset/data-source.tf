provider "mondoo" {}

data "mondoo_asset" "asset" {
  space_id = "my-space-1234567"
}

output "asset_mrns" {
  description = "MRNs of the assets"
  value       = [for asset in data.mondoo_asset.asset.assets : asset.mrn]
}

output "asset_names" {
  description = "Names of the assets"
  value       = [for asset in data.mondoo_asset.asset.assets : asset.name]
}