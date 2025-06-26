variable "org_id" {
  description = "The ID of the organization in which to create the spaces"
  type        = string
}

provider "mondoo" {}

resource "mondoo_space" "new_space" {
  name        = "My New Space"
  description = "A space used to secure my environment"
  org_id      = var.org_id

  # optional id otherwise it will be auto-generated
  # id = "your-space-id"
}

resource "mondoo_space" "custom_space" {
  name        = "My Custom Space"
  description = "A space used to secure my environment"
  org_id      = var.org_id

  # optional id otherwise it will be auto-generated
  id = "your-space-id"

  # All space settings are optional
  space_settings = {
    terminated_assets_configuration = {
      cleanup = true
    }
    unused_service_accounts_configuration = {
      cleanup = true
    }
    garbage_collect_assets_configuration = {
      enabled    = true
      after_days = 30
    }
    platform_vulnerability_configuration = {
      enabled = true
    }
    eol_assets_configuration = {
      enabled           = true
      months_in_advance = 6
    }
    cases_configuration = {
      auto_create        = false
      aggregation_window = 0
    }
  }
}
