terraform {
  required_version = ">= 1.3"

  required_providers {
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.38"
    }
  }
}

# Credentials come from the environment, for example MONDOO_CONFIG_PATH.
# See the "Authenticate" section of the repository README.
provider "mondoo" {
  region = var.region
}
