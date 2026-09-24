# Terraform Provider for Mondoo

> Status: It is currently in beta. Please report any issues you encounter.

The Mondoo provider lets you manage [Mondoo Platform](https://mondoo.com) with [Terraform](https://www.terraform.io/):
spaces, service accounts, policy and framework assignments, exceptions, custom policies and query packs, asset routing,
and integrations with cloud providers, SaaS platforms, and ticketing systems.

- **Provider documentation:** [registry.terraform.io/providers/mondoohq/mondoo](https://registry.terraform.io/providers/mondoohq/mondoo/latest/docs)
- **Mondoo documentation:** [mondoo.com/docs](https://mondoo.com/docs/platform/home/)
- **Examples:** [examples/](examples/) contains a working configuration for every resource and data source

## Quick start

### 1. Prerequisites

- [Terraform](https://developer.hashicorp.com/terraform/install) 1.0 or later
- A [Mondoo Platform account](https://mondoo.com/docs/platform/start/plat-start-acct/) and the ID of an
  [organization](https://mondoo.com/docs/platform/start/organize/overview/) in it. To find the ID, open the
  organization in the Mondoo Console and copy the value after `?organizationId=` in your browser's address bar.
- A [service account](https://mondoo.com/docs/platform/maintain/access/service_accounts/#generate-a-service-account-for-access-to-all-spaces-in-an-organization)
  with **Editor** permissions on that organization. Download its credentials as a JSON file.

### 2. Authenticate

The provider looks for credentials in this order and uses the first one it finds:

| Source                        | Use it when                                                                   |
|-------------------------------|-------------------------------------------------------------------------------|
| `MONDOO_CONFIG_BASE64`        | CI systems that store secrets as strings: the service account JSON, base64 encoded |
| `MONDOO_CONFIG_PATH`          | You have the service account JSON (or a workload identity config) on disk     |
| `MONDOO_API_TOKEN`            | You have a Mondoo API token                                                   |
| `credentials` provider field  | You want to pass the service account JSON from a Terraform variable           |
| Mondoo CLI config file        | You already ran `cnspec login` on this machine (`~/.config/mondoo/mondoo.yml`) |

For a first run, point the provider at the file you downloaded:

```shell
export MONDOO_CONFIG_PATH=~/Downloads/mondoo-service-account.json
```

### 3. Write a configuration

Create a `main.tf` that adds a space to your organization:

```terraform
terraform {
  required_providers {
    mondoo = {
      source = "mondoohq/mondoo"
    }
  }
}

provider "mondoo" {
  region = "us" # use "eu" if your organization is in the EU region
}

variable "org_id" {
  description = "The ID of your Mondoo organization"
  type        = string
}

resource "mondoo_space" "example" {
  name   = "Created by Terraform"
  org_id = var.org_id
}

output "space_id" {
  value = mondoo_space.example.id
}
```

### 4. Apply it

```shell
terraform init
terraform apply -var="org_id=your-org-id"
```

The new space appears in the Mondoo Console. Run `terraform destroy -var="org_id=your-org-id"` to remove it.

### Next steps

- Follow the [getting-started walkthroughs](examples/getting-started/) to enable policies, scan a machine, organize
  spaces per environment, give teams access, and write custom policies.
- Browse [examples/resources/](examples/resources/) for complete configurations, for example assigning policies with
  `mondoo_policy_assignment` or connecting AWS with `mondoo_integration_aws`.
- Read the [provider documentation](https://registry.terraform.io/providers/mondoohq/mondoo/latest/docs) for every
  resource's arguments and attributes.

### Troubleshooting

| Problem                                   | Fix                                                                                        |
|-------------------------------------------|--------------------------------------------------------------------------------------------|
| `No authentication found`                 | None of the sources in step 2 is set. Check that the variable is exported in the shell running Terraform. |
| `MONDOO_CONFIG_BASE64 must be a valid service account` | The value isn't base64. Encode the JSON file with `base64 < service-account.json`.  |
| Permission or "not found" errors on an organization | The service account needs **Editor** on the organization, not only on a space, and `org_id` must be the organization ID rather than its name. |
| Resources don't appear in the console     | Check `region`: an EU organization needs `region = "eu"`.                                  |

For more detail, including which credential source the provider picked, run Terraform with `TF_LOG=DEBUG`.

## Contributing

### Requirements

- [Go](https://go.dev/doc/install) at the version in [`go.mod`](go.mod)
- [Terraform](https://developer.hashicorp.com/terraform/install), which `make generate` uses to format the examples
- Optional: [golangci-lint](https://golangci-lint.run/) and [TFLint](https://github.com/terraform-linters/tflint)
  for `make lint` and `make hcl/lint`

### Project layout

| Path                   | Contents                                                                                    |
|------------------------|---------------------------------------------------------------------------------------------|
| `internal/provider/`   | Resources, data sources, and their acceptance tests (`*_test.go`)                          |
| `examples/`            | Example configurations. These are also embedded in the generated docs.                     |
| `templates/`           | Doc templates that override the generated pages, such as the provider index page           |
| `docs/`                | Generated documentation published to the Terraform Registry. **Don't edit it by hand.**    |
| `gen/`                 | Code generator for integration resources                                                   |

This provider is built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework).

### Build and run the provider locally

```shell
git clone https://github.com/mondoohq/terraform-provider-mondoo.git
cd terraform-provider-mondoo
make build
```

To try your local build with Terraform, enter development mode:

```shell
make dev/enter
```

This builds the provider into `~/.terraform.d/plugins` and writes a `~/.terraformrc` that makes Terraform use that
build instead of the Registry release. If you already have a `~/.terraformrc`, it is left unchanged, and you need to add
the `filesystem_mirror` block from [scripts/mirror-provider.sh](scripts/mirror-provider.sh) yourself. Now run
`terraform init` and `terraform apply` from any directory in [examples/](examples/). Run `make dev/enter` again after
each code change.

When you're done, run `make dev/exit`. **This deletes `~/.terraformrc`**, so back up the file first if you keep other
settings in it.

### Generate code and docs

After changing a resource schema or an example, run:

```shell
make generate
```

This regenerates the integration resources, formats the examples, and rebuilds `docs/` from the schemas, `examples/`,
and `templates/`. CI fails if `make generate` produces a diff, so commit its output.

### Test

The tests in `internal/provider/` are acceptance tests. They create and delete real resources in a Mondoo organization
and need an **organization-level** service account:

```shell
export MONDOO_CONFIG_BASE64=$(base64 < org-service-account.json)
make testacc
```

Use a dedicated test organization, because the tests create and delete spaces. Without credentials the test package
fails to start, which also affects `make test`. To run a single test, use
`make testacc TESTARGS="-run TestAccSpaceResource"`.

### Lint and format

```shell
make lint        # golangci-lint
make fmt         # gofmt
make hcl/fmt     # terraform fmt
make hcl/lint    # tflint
typos            # spell check, same as CI
```

### Add a resource

1. Add the resource in `internal/provider/`, following an existing resource of the same kind. The
   [Terraform Plugin Framework code generator](https://github.com/hashicorp/terraform-plugin-codegen-framework) can
   scaffold one: `tfplugingen-framework scaffold resource --name my_resource --output-dir internal/provider`.
2. Register it in `Resources()` in `internal/provider/provider.go`.
3. Add an example at `examples/resources/mondoo_<name>/resource.tf`, and an `import.sh` if the resource supports import.
4. Add an acceptance test, then run `make generate` and commit the updated `docs/`.

### Add a dependency

```shell
go get github.com/author/dependency
go mod tidy
```

Commit the changes to `go.mod` and `go.sum`.
