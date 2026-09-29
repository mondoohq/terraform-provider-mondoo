# 3. Team access

Give people and automation the access they need, and nothing more. This example sets up a security team that works
across the organization, an application team that works in one space, and a service account for a scheduled job.

**You'll create:** a [space](../../resources/mondoo_space/), two [teams](../../resources/mondoo_team/) and their
[members](../../resources/mondoo_team_member/), two [IAM bindings](https://registry.terraform.io/providers/mondoohq/mondoo/latest/docs/resources/iam_binding),
and a [service account](../../resources/mondoo_service_account/). With SSO, you can also create a
[team external group mapping](https://registry.terraform.io/providers/mondoohq/mondoo/latest/docs/resources/team_external_group_mapping).

> [!IMPORTANT]
> The security team and its role binding live on the organization, so the provider needs an **organization-level**
> service account, not one scoped to a single space.

## What each identity can do

| Identity                   | Where                | Roles                              | So they can                                     |
|----------------------------|----------------------|------------------------------------|-------------------------------------------------|
| `security` team            | The organization     | `viewer`, `policy-manager`         | See every space and manage policies everywhere  |
| `payments-app` team        | The new space        | `viewer`, `exceptions-requester`   | See their findings and request exceptions       |
| `reporting` service account| The new space        | `viewer`                           | Read results for a scheduled report             |

Access is granted by a `mondoo_iam_binding`, which connects an identity (a team, user, or service account) to a
resource (an organization, space, or workspace) with a list of roles. Teams themselves hold no permissions.

## Run it

```shell
cp terraform.tfvars.example terraform.tfvars   # set org_id and the team members' email addresses
terraform init
terraform apply
```

Open your organization in the Mondoo Console to see both teams and their members. People who don't have a Mondoo
account yet are pending members until they sign up.

## Use the service account

Service account credentials are a secret, so Terraform marks the output as sensitive. To hand them to a job, store them
in your CI system's secret store as `MONDOO_CONFIG_BASE64`:

```shell
terraform output -raw reporting_service_account
```

Anyone with access to the Terraform state can read these credentials. Keep the state in a
[backend](https://developer.hashicorp.com/terraform/language/backend) with encryption and access control, not in a
local file on a shared machine.

## Use SSO groups instead of email addresses

If your organization signs in through an OIDC identity provider, set `security_team_idp_group` to the name or ID of a
group from the provider's groups claim. Members of that group join the security team when they sign in, so you manage
membership in one place. You can then leave `security_team_members` empty.

## Clean up

```shell
terraform destroy
```

## Next

[4. Custom policies and exceptions](../04-custom-policy-and-exceptions/) adds your own checks and handles the ones that
don't apply to you.
