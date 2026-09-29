# One space per environment keeps findings, exceptions, and access separate,
# while for_each keeps the configuration in one place. Add an environment to
# var.environments and run terraform apply to create its space.
resource "mondoo_space" "env" {
  for_each = var.environments

  org_id      = var.org_id
  name        = title(each.key)
  description = each.value.description

  # Annotations are free-form key-value metadata attached to the space.
  annotations = {
    environment = each.key
    managed-by  = "terraform"
  }

  space_settings = {
    # Remove assets that haven't reported for this many days, such as
    # machines that were shut down.
    garbage_collect_assets_configuration = {
      enabled    = true
      after_days = each.value.remove_stale_assets_after
    }

    # Remove assets that are reported as terminated, such as deleted cloud
    # instances.
    terminated_assets_configuration = {
      cleanup = true
    }

    exceptions_configuration = {
      require_approval    = each.value.require_exception_approval
      allow_self_approval = false
    }
  }
}

# Every environment gets the common policies plus its own extras. distinct()
# drops duplicates if an environment repeats a common policy.
resource "mondoo_policy_assignment" "env" {
  for_each = var.environments

  scope_mrn = mondoo_space.env[each.key].mrn
  policies  = distinct(concat(var.common_policies, each.value.extra_policies))
  state     = "enabled"
}

# A long-lived token per environment, for example to store in the secret
# manager your provisioning tool reads when it installs cnspec on new machines.
resource "mondoo_registration_token" "env" {
  for_each = var.environments

  space_id    = mondoo_space.env[each.key].id
  description = "cnspec registration for ${each.key}"
  expires_in  = "8760h" # 365 days
}
