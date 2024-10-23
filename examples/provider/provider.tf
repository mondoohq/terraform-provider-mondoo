terraform {
  required_providers {
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.19"
    }
  }
}

provider "mondoo" {
  space  = "hungry-poet-1988"
  region = "us"
}
