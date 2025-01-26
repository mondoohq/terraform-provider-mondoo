---
page_title: "Provider: Mondoo"
description: |-
  Use the Mondoo provider to configure your Mondoo Platform infrastructure.
---

# Mondoo provider

Use the Mondoo provider to configure your Mondoo Platform infrastructure. To learn about Mondoo, read the [Mondoo documentation](https://mondoo.com/docs/platform/home/).

## Prerequisites

- A [Mondoo Platform account](https://mondoo.com/docs/platform/start/plat-start-acct/)

- An [organization](https://mondoo.com/docs/platform/start/organize/overview/) in your Mondoo Platform

- The ID of the organization

   To retrieve the ID: In the top navigation bar of the Mondoo Console, select the organization name. This opens the organization's overview page. In your browser's address bar, copy the value after `?organizationId`.

- A [Mondoo service account](https://mondoo.com/docs/platform/maintain/access/service_accounts/#generate-a-service-account-for-access-to-all-spaces-in-an-organization) with **Editor** permissions to the Mondoo organization

## Example

```terraform
terraform {
  required_providers {
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.21"
    }
  }
}

variable "org_id" {
  description = "The ID of the organization in which to create new spaces"
  type        = string
}

provider "mondoo" {}

data "mondoo_organization" "org" {
  id = var.org_id
}

resource "mondoo_space" "my_space" {
  name   = "My New Space"
  org_id = data.mondoo_organization.org.id
}

# Assign policies to the space

resource "mondoo_policy_assignment" "cis_policy_assignment_enabled" {
  space_id = mondoo_space.my_space.id

  policies = [
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-10-l1-ce",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-10-l1-bl",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-11-l1-ce",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-11-l1-bl",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2016-dc-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2016-ms-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2019-dc-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2019-ms-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2022-dc-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2022-ms-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-azure-windows-server-2019-dc-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-azure-windows-server-2019-ms-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-azure-windows-server-2022-dc-level-1",
    "//policy.api.mondoo.app/policies/cis-microsoft-azure-windows-server-2022-ms-level-1",
    "//policy.api.mondoo.app/policies/mondoo-edr-policy",
  ]

  state = "enabled"
}

# Set exceptions for Windows policies in the space
resource "mondoo_exception" "windows_defender_exception" {
  scope_mrn     = mondoo_space.my_space.mrn
  justification = "Windows Defender is disabled. Other EDR is used/configured instead."
  action        = "SNOOZE"
  check_mrns = [
    "//policy.api.mondoo.app/queries/cis-microsoft-windows-10--18.10.42.5.1",
    "//policy.api.mondoo.app/queries/cis-microsoft-windows-11--18.10.42.5.1",
    "//policy.api.mondoo.app/queries/cis-microsoft-windows-server-2016--18.10.42.5.1",
    "//policy.api.mondoo.app/queries/cis-microsoft-windows-server-2019--18.10.42.5.1",
    "//policy.api.mondoo.app/queries/cis-microsoft-windows-server-2022--18.10.42.5.1",
  ]
  depends_on = [
    mondoo_policy_assignment.cis_policy_assignment_enabled
  ]
}
```

## Apply the configuration to Mondoo Platform

1. To execute the Terraform configuration, set the `MONDOO_CONFIG_BASE64` environment variable with the Mondoo API token you created:

   ```bash
   export MONDOO_CONFIG_BASE64="your-token-here"
   ```

2. Initialize a working directory containing the Terraform configuration files:

   ```bash
   terraform init
   ```

3. Create an execution plan, which lets you preview the changes that the Terraform plan makes to your Mondoo organization:

   ```bash
   terraform plan -out plan.out
   ```

4. Execute the actions proposed in the Terraform plan.

   ```bash
   terraform apply -auto-approve plan.out
   ```

## Authentication

To configure the Mondoo provider you need a service account with **Editor** permissions, to create a service
account, see [Create and Manage Service Accounts](https://mondoo.com/docs/platform/maintain/access/service_accounts/).

By default, the provider uses the Mondoo CLI configuration file to authenticate to the Mondoo Platform. The CLI
configuration file is located at `~/.config/mondoo/mondoo.yml` on Linux and macOS, and `%HomePath%\mondoo\mondoo.yml`
on Windows.

You can alternatively use the following environment variables, ordered by precedence:

* `MONDOO_CONFIG_BASE64`
* `MONDOO_CONFIG_PATH`
* `MONDOO_API_TOKEN`

If you want to manage the credential as part of your Terraform configuration, use the `credentials` field:

```hcl
provider "mondoo" {
  credentials = "{json-formatted-credentials}"
}
```

## Regions

By default, the provider uses Mondoo Platform in the US region. To use the EU region, set the `region`
attribute:

```hcl
provider "mondoo" {
  region = "eu"
}
```

For dedicated Mondoo Platform installations, set the `endpoint` attribute:

```hcl
provider "mondoo" {
  endpoint = "https://api.{example.com}"
}
```

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `credentials` (String) The contents of a service account key file in JSON format.
- `endpoint` (String) The endpoint url of the server to manage resources
- `region` (String) The default region in which to manage resources. Valid regions are `us` or `eu`.
- `space` (String) The default space in which to manage resources.
