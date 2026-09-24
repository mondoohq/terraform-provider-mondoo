# Getting started

Four short walkthroughs that build on each other. Each one is a complete Terraform configuration that you can apply
against your own Mondoo organization, look at in the Mondoo Console, and destroy again. They only need Mondoo
credentials: no cloud account.

| Walkthrough                                                        | You'll learn to                                                                  | Time    |
|--------------------------------------------------------------------|----------------------------------------------------------------------------------|---------|
| [1. Your first space](01-first-space/)                             | Create a space, enable policies and a compliance framework, and scan a machine   | 10 min  |
| [2. One space per environment](02-space-per-environment/)          | Manage several spaces with different settings from one map with `for_each`       | 10 min  |
| [3. Team access](03-team-access/)                                  | Create teams, grant roles with IAM bindings, and set up a service account        | 10 min  |
| [4. Custom policies and exceptions](04-custom-policy-and-exceptions/) | Upload your own MQL checks and record a time-limited exception                 | 15 min  |

## Before you start

You need Terraform 1.3 or later and a service account for your Mondoo organization. The repository README's
[quick start](../../README.md#quick-start) walks through both, including how to find your organization ID.

Every walkthrough follows the same steps:

```shell
cd examples/getting-started/01-first-space
cp terraform.tfvars.example terraform.tfvars   # then edit it
terraform init
terraform apply
# ...look around in the Mondoo Console...
terraform destroy
```

Each walkthrough creates its own space, so they don't interfere with each other or with spaces you already have.
