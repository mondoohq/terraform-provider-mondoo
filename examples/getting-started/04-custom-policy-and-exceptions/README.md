# 4. Custom policies and exceptions

Mondoo's public policies cover common benchmarks, but every organization has rules of its own and checks that don't
apply to it. This example uploads a policy you write, enables it next to a CIS benchmark, and records a time-limited
exception for checks that don't fit your environment.

**You'll create:** a [space](../../resources/mondoo_space/), a [custom policy](../../resources/mondoo_custom_policy/)
from [`policies/ssh-hardening.mql.yaml`](policies/ssh-hardening.mql.yaml), a
[policy assignment](../../resources/mondoo_policy_assignment/), and an [exception](../../resources/mondoo_exception/).

## The custom policy

[`policies/ssh-hardening.mql.yaml`](policies/ssh-hardening.mql.yaml) holds three SSH checks written in
[MQL](https://mondoo.com/docs/mql/). Each check is a query that returns `true` when the asset passes. Test the policy
against your machine before you upload it:

```shell
cnspec scan local --policy-bundle policies/ssh-hardening.mql.yaml
```

Edit the file to add your own checks. Give each policy and check its own `uid`, and keep a uid stable once it's in
use, because exceptions and results refer to it.

## The exception

The CIS Windows Server 2022 benchmark expects Microsoft Defender Antivirus to be on. If you run a different EDR product,
that check fails on every server even though you're protected. An exception with `RISK_ACCEPTED` records the reason,
stops the check from counting against your score, and expires on `exception_valid_until` so someone reviews the
decision.

| `action`         | Use it when                                                         | `valid_until` |
|------------------|---------------------------------------------------------------------|---------------|
| `RISK_ACCEPTED`  | The check fails and you accept the risk, usually for a limited time | Required      |
| `WORKAROUND`     | The check fails, but another control addresses the risk             | Required      |
| `FALSE_POSITIVE` | The check reports a problem that doesn't exist                      | Required      |
| `DISABLE`        | The check doesn't apply to you, so don't run it                     | Not allowed   |

When you set one of the first three actions, the provider requires `valid_until`, so every accepted risk has a review
date. `SNOOZE` also still works but is deprecated: use `RISK_ACCEPTED` with a `valid_until` date instead.

`scope_mrn` controls where the exception applies. This example uses the space, so it covers every asset in it. To limit
an exception to one asset, use that asset's MRN instead. The [`mondoo_assets`](../../data-sources/mondoo_assets/)
data source can look it up.

## Run it

```shell
cp terraform.tfvars.example terraform.tfvars   # set org_id and a future exception_valid_until
terraform init
terraform apply
```

Then change a check in `policies/ssh-hardening.mql.yaml` and run `terraform plan`. Terraform detects the edit and plans
to upload the new version.

## Clean up

```shell
terraform destroy
```

## Where to go from here

- Connect your infrastructure with an integration, such as [AWS](../../resources/mondoo_integration_aws_serverless/),
  [Azure](../../resources/mondoo_integration_azure/), [Google Cloud](../../resources/mondoo_integration_gcp/), or
  [GitHub](../../resources/mondoo_integration_github/).
- Send findings to the tools your teams use, such as [Jira](../../resources/mondoo_integration_jira/) or
  [Slack](../../resources/mondoo_integration_slack/).
- Browse [all examples](../../README.md#find-an-example).
