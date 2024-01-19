# Terraform Provider Mondoo

> Status: It is currently in beta. Please report any issues you encounter.

The Mondoo Provider allows [Terraform](https://terraform.io/) to manage [Mondoo](https://mondoo.com) resources.

## Provider Usage

```terraform
terraform {
  required_providers {
    mondoo = {
      source = "mondoohq/mondoo"
    }
  }
}

provider "mondoo" {
  region = "us" # use "eu" for the European region
}
```

## Developing the provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (see [Requirements](#requirements) above).

To compile the provider, run `go install`. This will build the provider and put the provider binary in the `$GOPATH/bin` directory.

To generate or update documentation, run `go generate`.

In order to run the full suite of Acceptance tests, run `make testacc`.

_Note:_ Acceptance tests create real resources, and often cost money to run.

```shell
make testacc
```

This provider is built on the [Terraform Plugin Framework](https://github.com/hashicorp/terraform-plugin-framework). The template repository built on the [Terraform Plugin SDK](https://github.com/hashicorp/terraform-plugin-sdk) can be found at [terraform-provider-scaffolding](https://github.com/hashicorp/terraform-provider-scaffolding). The directory structure contains the following directories:

- A resource and a data source (`internal/provider/`),
- Examples (`examples/`) and generated documentation (`docs/`),
- Miscellaneous meta files.

### Requirements

- [Terraform](https://developer.hashicorp.com/terraform/downloads) >= 1.0
- [Go](https://golang.org/doc/install) >= 1.19

### Building The Provider

1. Clone the repository
1. Enter the repository directory
1. Build the provider using the Go `install` command:

```shell
go install
```

### Adding Dependencies

This provider uses [Go modules](https://github.com/golang/go/wiki/Modules).
Please see the Go documentation for the most up to date information about using Go modules.

To add a new dependency `github.com/author/dependency` to your Terraform provider:

```shell
go get github.com/author/dependency
go mod tidy
```

Then commit the changes to `go.mod` and `go.sum`.

### Using the provider

After building the provider please install the terraform provider:

```
# ARCH = 'linux_amd64'
export ARCH='darwin_arm64'
export VERSION='1.0.0'

mkdir -p ~/.terraform.d/plugins/registry.terraform.io/mondoo/mondoo/$VERSION/$ARCH
cp $GOPATH/bin/terraform-provider-mondoo ~/.terraform.d/plugins/registry.terraform.io/mondoo/mondoo/$VERSION/$ARCH/
```
