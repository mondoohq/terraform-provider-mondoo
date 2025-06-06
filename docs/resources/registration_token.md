page_title: "mondoo_registration_token Resource - terraform-provider-mondoo"
subcategory: ""
description: |-
  Registration Token resource
---

# mondoo_registration_token

Registration Token resource

## Example Usage

```terraform
# Variables
# ----------------------------------------------

variable "space_names" {
  description = "Create Spaces with these names"
  type        = list(string)
  default     = []
}

variable "org_id" {
  description = "The ID of the organization in which to create the spaces"
  type        = string
  default     = ""
}

# Configure the Mondoo
# ----------------------------------------------

provider "mondoo" {}

resource "mondoo_space" "my_space" {
  count  = length(var.space_names)
  name   = var.space_names[count.index]
  org_id = var.org_id
}

resource "mondoo_registration_token" "token" {
  description   = "Get a mondoo registration token"
  count         = length(var.space_names)
  space_id      = mondoo_space.my_space[count.index].id
  no_expiration = true
  # define optional expiration
  # expires_in = "1h"
  depends_on = [
    mondoo_space.my_space
  ]
}

output "space_registration_token" {
  description = "The list of space registration tokens for the specified spaces"
  value = [
    for count, space in mondoo_space.my_space :
    {
      space-name : space.name,
      space-id : space.id,
      token : mondoo_registration_token.token[count].result
    }
  ]
  sensitive = true
}
```

## Example to Create Spaces and Get Registration Tokens

This example demonstrates how to create three different Mondoo Spaces in a Mondoo Organization and obtain a non-expiring
Mondoo Registration Token for each Space.

**Prerequisites**

Before proceeding, make sure you have the following:

- [Mondoo Platform account](https://mondoo.com/docs/platform/start/plat-start-acct/)
- [Mondoo Organization](https://mondoo.com/docs/platform/start/organize/overview/)
- [Mondoo API Token](https://mondoo.com/docs/platform/maintain/access/api-tokens/)

**Usage**

1. Adjust the variables `space_names` and `org_id`  in the  `terraform.tfvars` file:

```hcl
space_names = ["Terraform Mondoo1", "Terraform Mondoo2", "Terraform Mondoo3"]
org_id      = "love-mondoo-131514041515"
```

2. Set the Mondoo Organization Service Account token

```bash
export MONDOO_CONFIG_BASE64=""
```

3. Initialize a working directory containing Terraform configuration files:

```bash
terraform init

Initializing the backend...

Initializing provider plugins...
- Finding latest version of mondoo/mondoo...
- Installing mondoo/mondoo v1.0.0...
- Installed mondoo/mondoo v1.0.0 (unauthenticated)

Terraform has created a lock file .terraform.lock.hcl to record the provider
selections it made above. Include this file in your version control repository
so that Terraform can guarantee to make the same selections by default when
you run "terraform init" in the future.

...

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
```

4. Create an execution plan to preview the changes that the Terraform plan will make to your Mondoo Organization:

```bash
terraform plan -out plan.out

Terraform used the selected providers to generate the following execution plan. Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # mondoo_registration_token.token[0] will be created
  + resource "mondoo_registration_token" "token" {
      + description   = "Get a mondoo registration token"
      + expires_at    = (known after apply)
      + mrn           = (known after apply)
      + no_expiration = true
      + result        = (sensitive value)
      + revoked       = (known after apply)
      + space_id      = (known after apply)
    }

  # mondoo_registration_token.token[1] will be created
  + resource "mondoo_registration_token" "token" {
      + description   = "Get a mondoo registration token"
      + expires_at    = (known after apply)
      + mrn           = (known after apply)
      + no_expiration = true
      + result        = (sensitive value)
      + revoked       = (known after apply)
      + space_id      = (known after apply)
    }

  # mondoo_registration_token.token[2] will be created
  + resource "mondoo_registration_token" "token" {
      + description   = "Get a mondoo registration token"
      + expires_at    = (known after apply)
      + mrn           = (known after apply)
      + no_expiration = true
      + result        = (sensitive value)
      + revoked       = (known after apply)
      + space_id      = (known after apply)
    }

  # mondoo_space.my_space[0] will be created
  + resource "mondoo_space" "my_space" {
      + id     = (known after apply)
      + name   = "Terraform Mondoo1"
      + org_id = "love-mondoo-131514041515"
    }

  # mondoo_space.my_space[1] will be created
  + resource "mondoo_space" "my_space" {
      + id     = (known after apply)
      + name   = "Terraform Mondoo2"
      + org_id = "love-mondoo-131514041515"
    }

  # mondoo_space.my_space[2] will be created
  + resource "mondoo_space" "my_space" {
      + id     = (known after apply)
      + name   = "Terraform Mondoo3"
      + org_id = "love-mondoo-131514041515"
    }

Plan: 6 to add, 0 to change, 0 to destroy.

Changes to Outputs:
  + complete_space_setup = (sensitive value)

Saved the plan to: plan.out

To perform exactly these actions, run the following command to apply:
    terraform apply "plan.out"
```

5. Apply the actions proposed in the Terraform plan:

```bash
terraform apply -auto-approve plan.out

mondoo_space.my_space[2]: Creating...
mondoo_space.my_space[1]: Creating...
mondoo_space.my_space[0]: Creating...
mondoo_space.my_space[1]: Creation complete after 1s [id=admiring-wiles-299863]
mondoo_space.my_space[2]: Creation complete after 1s [id=inspiring-tesla-178593]
mondoo_space.my_space[0]: Creation complete after 1s [id=sad-wescoff-418523]
mondoo_registration_token.token[2]: Creating...
mondoo_registration_token.token[0]: Creating...
mondoo_registration_token.token[1]: Creating...
mondoo_registration_token.token[0]: Creation complete after 0s
mondoo_registration_token.token[1]: Creation complete after 0s
mondoo_registration_token.token[2]: Creation complete after 0s

Apply complete! Resources: 6 added, 0 changed, 0 destroyed.

Outputs:

complete_space_setup = <sensitive>
```

6. Extract the value of the output variable `complete_space_setup` from the state file:

```bash
terraform output -json complete_space_setup | jq

[
  {
    "space-id": "sad-wescoff-418523",
    "space-name": "Terraform Mondoo1",
    "token": "eyJhbGciOiJFUzM4NCIsInR...XIPlutxBAhNHar"
  },
  {
    "space-id": "admiring-wiles-299863",
    "space-name": "Terraform Mondoo2",
    "token": "eyJhbGciOiJFUzM4NCIsI...5zXkvADk_KBpXgfIHS3rXQXJIK"
  },
  {
    "space-id": "inspiring-tesla-178593",
    "space-name": "Terraform Mondoo3",
    "token": "eyJhbGciOiJFUzM4...OoeQwc-TjglUZx"
  }
]
```

You successfully created Mondoo spaces and generated registration tokens for each space, which will be displayed in the
output.

<!-- schema generated by tfplugindocs -->
## Schema

### Optional

- `description` (String) Description of the token.
- `expires_at` (String) The date and time when the token will expire.
- `expires_in` (String) The duration after which the token will expire. Format: 1h, 1d, 1w, 1m, 1y
- `no_expiration` (Boolean) If set to true, the token will not expire.
- `revoked` (Boolean) If set to true, the token is revoked.
- `space_id` (String) Identifier of the Mondoo space in which to create the token. If there is no space ID, the provider space is used.

### Read-Only

- `mrn` (String) The Mondoo Resource Name (MRN) of the created token.
- `result` (String, Sensitive) The generated token.