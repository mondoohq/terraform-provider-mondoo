# 1. Your first space

Create a space, turn on security policies and a compliance framework, and scan a machine into it. By the end you'll
have real findings in the Mondoo Console that Terraform set up.

**You'll create:** a [space](../../resources/mondoo_space/), a [policy assignment](../../resources/mondoo_policy_assignment/),
a [framework assignment](../../resources/mondoo_framework_assignment/), and a
[registration token](../../resources/mondoo_registration_token/) that expires after 24 hours.

## Before you start

- Complete [authentication](../../../README.md#2-authenticate) so the provider can reach your organization.
- Install [cnspec](https://github.com/mondoohq/cnspec#installation) if you want to scan a machine in step 3.

## 1. Configure

```shell
cp terraform.tfvars.example terraform.tfvars
```

Set `org_id` to your organization ID, and `region = "eu"` if your organization is in the EU region.

## 2. Apply

```shell
terraform init
terraform apply
```

Terraform shows the four resources it will create. Type `yes` to create them. When it finishes, the new space is in
the Mondoo Console with three policies and CIS Controls v8 enabled, but no assets yet.

## 3. Scan a machine into the space

Register this machine with the token Terraform created, then scan it:

```shell
cnspec login --token "$(terraform output -raw registration_token)"
cnspec scan local
```

Open the space in the Mondoo Console. The machine is listed under its inventory, with results for the policy that
matches its operating system, and the CIS Controls framework shows how those results map to controls.

> [!NOTE]
> `cnspec login` saves its configuration on this machine, and future scans report to this space. To report to a
> different space later, run `cnspec login` again with another space's token.

## 4. Change something

Terraform now owns the space's configuration. Try changing it:

- Remove a policy from `policies` in `terraform.tfvars` and run `terraform apply`. The policy is disabled in the space.
- Add another framework, such as `"//policy.api.mondoo.app/frameworks/iso-27001-2022"`, to `frameworks`.

## 5. Clean up

```shell
terraform destroy
```

This deletes the space and everything in it, including the scanned assets.

## Next

[2. One space per environment](../02-space-per-environment/) creates several spaces from one configuration.
