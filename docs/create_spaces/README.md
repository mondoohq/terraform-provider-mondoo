# Example to create spaces + get registration token

This example creates 3 different Mondoo Spaces in a given Mondoo Organisation and provides the user for each Space a non-expiring Mondoo Registration Token.

## Prereqs

- [Mondoo Platform account](https://mondoo.com/docs/platform/start/plat-start-acct/)
- [Mondoo Organisation](https://mondoo.com/docs/platform/start/organize/overview/)
- [Mondoo API Token](https://mondoo.com/docs/platform/maintain/access/api-tokens/)

## Usage

Adjust the variables `space_names` and `org_id` in `terraform.tfvars`:

```coffee
space_names = ["Terraform Mondoo1", "Terraform Mondoo2", "Terraform Mondoo3"]
org_id = "love-mondoo-131514041515"
```

Set the Mondoo API token

```bash
export MONDOO_API_TOKEN="InsertTokenHere"
```

Initialize a working directory containing Terraform configuration files.

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

╷
│ Warning: Incomplete lock file information for providers
│
│ Due to your customized provider installation methods, Terraform was forced to calculate lock file checksums locally for the following providers:
│   - mondoo/mondoo
│
│ The current .terraform.lock.hcl file only includes checksums for darwin_arm64, so Terraform running on another platform will fail to install these providers.
│
│ To calculate additional checksums for another platform, run:
│   terraform providers lock -platform=linux_amd64
│ (where linux_amd64 is the platform to generate)
╵

Terraform has been successfully initialized!

You may now begin working with Terraform. Try running "terraform plan" to see
any changes that are required for your infrastructure. All Terraform commands
should now work.

If you ever set or change modules or backend configuration for Terraform,
rerun this command to reinitialize your working directory. If you forget, other
commands will detect it and remind you to do so if necessary.
```

Create an execution plan, which lets you preview the changes that the Terraform plan makes to your Mondoo Organisation:

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
      + no_exipration = true
      + result        = (sensitive value)
      + revoked       = (known after apply)
      + space_id      = (known after apply)
    }

  # mondoo_registration_token.token[1] will be created
  + resource "mondoo_registration_token" "token" {
      + description   = "Get a mondoo registration token"
      + expires_at    = (known after apply)
      + mrn           = (known after apply)
      + no_exipration = true
      + result        = (sensitive value)
      + revoked       = (known after apply)
      + space_id      = (known after apply)
    }

  # mondoo_registration_token.token[2] will be created
  + resource "mondoo_registration_token" "token" {
      + description   = "Get a mondoo registration token"
      + expires_at    = (known after apply)
      + mrn           = (known after apply)
      + no_exipration = true
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

────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────

Saved the plan to: plan.out

To perform exactly these actions, run the following command to apply:
    terraform apply "plan.out"
```

Execute the actions proposed in the Terraform plan

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

Extract the value of the output variable `complete_space_setup` from the state file.

```bash
terraform output -json complete_space_setup | jq

[
  {
    "space-id": "sad-wescoff-418523",
    "space-name": "Terraform Mondoo1",
    "token": "eyJhbGciOiJFUzM4NCIsInR5cCI6IkpXVCJ9.eyJhcGlfZW5kcG9pbnQiOiJodHRwczovL3VzLmFwaS5tb25kb28uY29tIiwiYXVkIjpbIm1vbmRvbyJdLCJjZXJ0X3ZhbGlkX3VudGlsIjoiOTk5OS0xMi0zMVQyMzo1OTo1OVoiLCJkZXNjIjoiR2V0IGEgbW9uZG9vIHJlZ2lzdHJhdGlvbiB0b2tlbiIsImlhdCI6MTY5OTA5NDA3MiwiaXNzIjoibW9uZG9vL2FtcyIsImxhYmVscyI6bnVsbCwibmJmIjoxNjk5MDk0MDcyLCJvd25lciI6IiIsInNjb3BlIjoiLy4NTI1Iiwic3ViIjoiLy9hZ2VudHMuYXBpLm1vbmRvby5hcHAvb3JnYW5pemF0aW9ucy9zdHVwZWZpZWQtam9obnNvbi02MzExNTUvc2VydmljZWFjY291bnRzLzJYZmxFU3NJN3VPbHc2VVhUMXlsbXdhUGRrciJ9.ajcJeYC5WTX7TwJdIO8wBITXwIGHuhxp2qGVgAWKaRgKTUlbEUkua898PBJWpseDDUpRZVKMBZpQjd78xglJtd0nUiBvg2b4py3XIPlutxBAhNHar"
  },
  {
    "space-id": "admiring-wiles-299863",
    "space-name": "Terraform Mondoo2",
    "token": "eyJhbGciOiJFUzM4NCIsInR5cCI6IkpXVCJ9.eyJhcGlfZW5kcG9pbnQiOiJodHRwczovL3VzHJhdGlvbiB0b2tlbiIsImlhdCI6MTY5OTA5NDA3MiwiaXNzIjoibW9uZG9vL2FtcyIsImxhYmVscyI6bnVsbCwibmJmIjoxNjk5MDk0MDcyLCJvd25lciI6IiIsInNjb3BlIjoiLy9jYXB0YWluLmFwaS5tb25kb28uYXBwL3NwYWNlcy9hZG1pcmluZy13aWxlcy0yOTk4NjQiLCJzcGFjZSI6Ii8vY2FwdGFpbi5hcGkubW9uZG9vLmFwcC9zcGFjZXMvYWRtaXJpbmctd2lsZXMtMjk5ODY0Iiwic3ViIjoiLy9hZ2VudHMuYXBpLm1vbmRvby5hcHAvb3JnYW5pemF0aW9ucy9zdHVwZWZpZWQtam9obnNvbi02MzExNTUvc2VydmljZWFjY291bnRzLzJYZmxFU3NJN3VPbHc2VVhUMXlsbXdhUGRrciJ9.Dq98j1sWXShNxhWXJC0aqZsbcqcOyDH3SQdwU7S67bh_qQMgYS8WSQgM_0QmbVNOBYg3mNVEr2lwB45w105zXkvADk_KBpXgfIHS3rXQXJIK"
  },
  {
    "space-id": "inspiring-tesla-178593",
    "space-name": "Terraform Mondoo3",
    "token": "eyJhbGciOiJFUzM4NCIsInR5cCI6IkpXVCJ9.eyJhcGlfZW5kcG9pbnQiOiJodHRwczovL3VzLmFwaS5tb25kb28uY29tIiwiYXVkIjpbIm1vbmRvbyJdLCJjZXJ0X3ZhbGlkX3VudGlsIjoiOTk5OS0xMi0zMVQyMzo1OTo1OVoiLCJkZXNjIjoiR2V0IGEgbW9uZG9vIHJlZ2lzdHJhdGlvbiB0b2tlbiIsImlhdCI6MTY5OTA5NDA3MiwiaXNzIjoibW9uZG9vL2FtcyIsImxhYmVscyI6bnVsbCwibmJmIjoxNjk5MDk0MDcyLCJvd25lciI6IiIsInNjb3BlIjopcmluZy10ZXNsYS0xNzg1OTIiLCJzdWIiOiIvL2FnZW50cy5hcGkubW9uZG9vLmFwcC9vcmdhbml6YXRpb25zL3N0dXBlZmllZC1qb2huc29uLTYzMTE1NS9zZXJ2aWNlYWNjb3VudHMvMlhmbEVTc0k3dUmaFeCIKxr6xbSDqNRIzEwSDVlx7TO2AVQm9w-k0hy8jCkfjXk6VBGwFOtz9TiWHeoQZz8igh5pOoeQwc-TjglUZx"
  }
]
```