# 2. One space per environment

Create a production, a staging, and a development space from a single map, each with its own policies, asset
clean-up rules, and exception rules. This is a common way to organize Mondoo: findings, exceptions, and access stay
separate per environment, and adding an environment is one more entry in a variable.

**You'll create:** three [spaces](../../resources/mondoo_space/), a
[policy assignment](../../resources/mondoo_policy_assignment/) in each, and a
[registration token](../../resources/mondoo_registration_token/) for each.

**Terraform concepts:** [`for_each`](https://developer.hashicorp.com/terraform/language/meta-arguments/for_each) over a
map, and object variables with
[optional attributes](https://developer.hashicorp.com/terraform/language/expressions/type-constraints#optional-object-type-attributes).

## How it works

Everything is driven by `var.environments`:

| Field                        | Default | What it controls                                                        |
|------------------------------|---------|-------------------------------------------------------------------------|
| `description`                | —       | The space's description (required)                                      |
| `extra_policies`             | `[]`    | Policies enabled in this environment on top of `var.common_policies`    |
| `remove_stale_assets_after`  | `30`    | Days before an asset that stopped reporting is removed                  |
| `require_exception_approval` | `false` | Whether exceptions need approval from someone other than the requester  |

In the defaults, production also enables the EDR policy, keeps assets for 90 days, and requires approval for
exceptions. Development removes assets after 7 days because sandboxes come and go.

## Run it

```shell
cp terraform.tfvars.example terraform.tfvars   # then set org_id
terraform init
terraform apply
```

Terraform creates nine resources. List the new spaces:

```shell
terraform output spaces
```

To scan a machine into one environment, use that environment's token:

```shell
cnspec login --token "$(terraform output -json registration_tokens | jq -r .staging)"
cnspec scan local
```

## Add an environment

Add an entry to `environments` in `terraform.tfvars`. Setting the variable replaces the whole default map, so include
the environments you want to keep:

```hcl
environments = {
  production = {
    description                = "Production workloads."
    require_exception_approval = true
  }
  qa = {
    description = "Quality assurance."
  }
}
```

`terraform apply` then creates the `qa` space and **destroys** the staging and development spaces, because they're no
longer in the map. Read the plan before you confirm.

## Clean up

```shell
terraform destroy
```

## Next

[3. Team access](../03-team-access/) gives people and automation access to spaces like these.
