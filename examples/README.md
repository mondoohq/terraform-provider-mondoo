# Examples

Terraform configurations for the Mondoo provider, from a first space to a full organization setup.

**New to the provider?** Start with [getting-started/](getting-started/): four short walkthroughs that you can apply
against your own organization and destroy again afterwards. If you haven't set up credentials yet, do the
[quick start](../README.md#quick-start) in the repository README first.

**Looking for a specific resource?** Find it [by task](#find-an-example) below. The example in each resource's
directory is also the one on its page in the
[provider documentation](https://registry.terraform.io/providers/mondoohq/mondoo/latest/docs).

## What's in this directory

| Directory                              | Contents                                                                                   |
|----------------------------------------|--------------------------------------------------------------------------------------------|
| [getting-started/](getting-started/)   | Guided, end-to-end configurations, each with a README that explains what it does and why  |
| [resources/](resources/)               | One example per resource, in `resources/<resource name>/resource.tf`                       |
| [data-sources/](data-sources/)         | One example per data source, in `data-sources/<data source name>/data-source.tf`           |
| [provider/](provider/)                 | The example on the provider's documentation index page                                     |

## Run an example

Every example is a working configuration. To run one from `resources/` or `data-sources/`:

1. Set up [authentication](../README.md#2-authenticate), for example `export MONDOO_CONFIG_PATH=~/mondoo-sa.json`.
2. `cd` into the example's directory and replace the placeholders:

   | Placeholder                         | Replace it with                                                                  |
   |-------------------------------------|----------------------------------------------------------------------------------|
   | `hungry-poet-123456`                | The ID of one of your spaces. Several examples set it as the provider's `space`. |
   | `your-org-id`, `var.org_id`         | Your organization ID                                                             |
   | `//captain.api.mondoo.app/...`      | The MRN of your own space, organization, or integration                          |
   | Cloud account IDs, tokens, and keys | Your own values. Examples read secrets from variables, never from the file.      |

3. Run `terraform init` and `terraform apply`. Variables without a default are prompted for, or you can set them in a
   `terraform.tfvars` file.

Git ignores the state and variable files that Terraform creates. When you're done, run `terraform destroy`, and
`make cleanup-examples` from the repository root removes the leftover files.

Integration examples often use a second provider, such as `aws`, `azurerm`, or `google`, to create the cloud-side
resources Mondoo needs. Those examples need credentials for that cloud too.

## Find an example

### Organize assets

| Resource or data source                                                    | Use it to                                                                       |
|----------------------------------------------------------------------------|---------------------------------------------------------------------------------|
| [`mondoo_organization`](resources/mondoo_organization/)                    | Create an organization, the top-level container for spaces                      |
| [`mondoo_space`](resources/mondoo_space/)                                  | Create a space and configure how it handles stale assets, EOL, and exceptions   |
| [`mondoo_workspace`](resources/mondoo_workspace/)                          | Group assets within a space by conditions, such as a label or a risk rating     |
| [`mondoo_asset_routing_rule`](resources/mondoo_asset_routing_rule/)        | Send assets to the right space by rule, one rule at a time                      |
| [`mondoo_asset_routing_table`](resources/mondoo_asset_routing_table/)      | Manage all asset routing rules for an organization as one ordered list          |
| [`mondoo_resource_contacts`](resources/mondoo_resource_contacts/)          | Record who to contact about an organization, space, or workspace                |
| [`mondoo_organization`](data-sources/mondoo_organization/) (data source)   | Look up an organization and list its spaces                                     |
| [`mondoo_space`](data-sources/mondoo_space/) (data source)                 | Look up an existing space by ID or MRN                                          |
| [`mondoo_assets`](data-sources/mondoo_assets/) (data source)               | List the assets in a space, for example to scope an exception to one asset      |

### Control access

| Resource                                                                           | Use it to                                                                    |
|------------------------------------------------------------------------------------|------------------------------------------------------------------------------|
| [`mondoo_team`](resources/mondoo_team/)                                            | Create teams and grant them roles with `mondoo_iam_binding`. Also shows `mondoo_team_external_group_mapping`. |
| [`mondoo_team_member`](resources/mondoo_team_member/)                              | Add people to a team by email address                                        |
| [`mondoo_service_account`](resources/mondoo_service_account/)                      | Create credentials for automation, such as CI jobs                          |
| [`mondoo_registration_token`](resources/mondoo_registration_token/)                | Create tokens that register machines in a space with cnspec                 |
| [`mondoo_iam_workload_identity_binding`](resources/mondoo_iam_workload_identity_binding/) | Let CI systems such as GitHub Actions authenticate without a stored secret |
| [`mondoo_scim_group_mapping`](resources/mondoo_scim_group_mapping/)                | Map SCIM groups from your identity provider to organization and space roles |

### Policies and compliance

| Resource or data source                                                  | Use it to                                                                         |
|--------------------------------------------------------------------------|-----------------------------------------------------------------------------------|
| [`mondoo_policy_assignment`](resources/mondoo_policy_assignment/)        | Enable policies in a space or organization                                        |
| [`mondoo_querypack_assignment`](resources/mondoo_querypack_assignment/)  | Enable query packs, which collect data without pass/fail scoring                 |
| [`mondoo_framework_assignment`](resources/mondoo_framework_assignment/)  | Enable compliance frameworks, such as CIS Controls or ISO 27001                   |
| [`mondoo_custom_policy`](resources/mondoo_custom_policy/)                | Upload your own policy written in MQL                                             |
| [`mondoo_custom_querypack`](resources/mondoo_custom_querypack/)          | Upload your own query pack                                                        |
| [`mondoo_custom_framework`](resources/mondoo_custom_framework/)          | Upload your own compliance framework                                              |
| [`mondoo_exception`](resources/mondoo_exception/)                        | Accept, snooze, or disable specific checks or vulnerabilities                     |
| [`mondoo_policies`](data-sources/mondoo_policies/) (data source)         | List the policies and query packs available in, or enabled in, a space           |
| [`mondoo_frameworks`](data-sources/mondoo_frameworks/) (data source)     | List the compliance frameworks in a space                                         |

### Scan infrastructure

| Platform                 | Example                                                                                                           |
|--------------------------|-------------------------------------------------------------------------------------------------------------------|
| AWS                      | [`mondoo_integration_aws_serverless`](resources/mondoo_integration_aws_serverless/) (scanner runs in your AWS account) or [`mondoo_integration_aws`](resources/mondoo_integration_aws/) |
| Microsoft Azure          | [`mondoo_integration_azure`](resources/mondoo_integration_azure/)                                                 |
| Google Cloud             | [`mondoo_integration_gcp`](resources/mondoo_integration_gcp/) or [`mondoo_integration_gcp_serverless`](resources/mondoo_integration_gcp_serverless/) (scanner runs in your project) |
| Oracle Cloud             | [`mondoo_integration_oci_tenant`](resources/mondoo_integration_oci_tenant/)                                       |
| Domains and endpoints    | [`mondoo_integration_domain`](resources/mondoo_integration_domain/) (TLS and HTTP), [`mondoo_integration_shodan`](resources/mondoo_integration_shodan/) (external exposure) |

### Scan SaaS and developer platforms

| Platform                 | Example                                                                                              |
|--------------------------|------------------------------------------------------------------------------------------------------|
| GitHub                   | [`mondoo_integration_github`](resources/mondoo_integration_github/)                                  |
| GitLab                   | [`mondoo_integration_gitlab`](resources/mondoo_integration_gitlab/)                                  |
| Azure DevOps             | [`mondoo_integration_azure_devops`](resources/mondoo_integration_azure_devops/)                      |
| Microsoft 365            | [`mondoo_integration_ms365`](resources/mondoo_integration_ms365/)                                    |
| Microsoft Intune         | [`mondoo_integration_ms_intune`](resources/mondoo_integration_ms_intune/)                            |
| Google Workspace         | [`mondoo_integration_google_workspace`](resources/mondoo_integration_google_workspace/)              |
| Okta                     | [`mondoo_integration_okta`](resources/mondoo_integration_okta/)                                      |
| Slack                    | [`mondoo_integration_slack`](resources/mondoo_integration_slack/)                                    |

### Connect security tools

| Tool                          | Example                                                                                  |
|-------------------------------|------------------------------------------------------------------------------------------|
| CrowdStrike Falcon            | [`mondoo_integration_crowdstrike`](resources/mondoo_integration_crowdstrike/)            |
| SentinelOne                   | [`mondoo_integration_sentinel_one`](resources/mondoo_integration_sentinel_one/)          |
| Microsoft Defender for Cloud  | [`mondoo_integration_msdefender`](resources/mondoo_integration_msdefender/)              |

### Send findings elsewhere

| Destination              | Example                                                                                                                  |
|--------------------------|--------------------------------------------------------------------------------------------------------------------------|
| Jira                     | [`mondoo_integration_jira`](resources/mondoo_integration_jira/): create and close issues from findings                   |
| Zendesk                  | [`mondoo_integration_zendesk`](resources/mondoo_integration_zendesk/): create tickets from findings                      |
| Email                    | [`mondoo_integration_email`](resources/mondoo_integration_email/): send tickets to any email address or ticket system    |
| SIEM                     | [`mondoo_integration_audit_log_export`](resources/mondoo_integration_audit_log_export/): audit logs in OCSF format       |
| Data warehouses and buckets | [`mondoo_export_bigquery`](https://registry.terraform.io/providers/mondoohq/mondoo/latest/docs/resources/export_bigquery), [`mondoo_export_s3`](https://registry.terraform.io/providers/mondoohq/mondoo/latest/docs/resources/export_s3), and [`mondoo_export_gcs_bucket`](https://registry.terraform.io/providers/mondoohq/mondoo/latest/docs/resources/export_gcs_bucket). Their examples are in the provider documentation. |

## Contributing examples

The documentation generator, [tfplugindocs](https://github.com/hashicorp/terraform-plugin-docs), embeds these files
in the provider documentation:

| File                                              | Appears on                        |
|---------------------------------------------------|-----------------------------------|
| `provider/provider.tf`                            | The provider index page           |
| `resources/<resource name>/resource.tf`           | The resource's page               |
| `resources/<resource name>/import.sh`             | The resource's "Import" section   |
| `data-sources/<data source name>/data-source.tf`  | The data source's page            |

Other files, such as `main.tf` with the `required_providers` block or helper files like `policy.mql.yaml`, make an
example runnable but aren't shown in the documentation. `getting-started/` isn't shown in the documentation at all.

When you add or change an example:

- Keep `resource.tf` short and focused on its resource, with comments that explain *why* a value is set, and read
  secrets from `sensitive` variables.
- Use `hungry-poet-123456` as the placeholder space ID and `//captain.api.mondoo.app/...` MRNs, so readers recognize
  what to replace.
- Run `make generate` to format the examples and regenerate `docs/`, and `make hcl/lint` to run TFLint. CI runs both.
