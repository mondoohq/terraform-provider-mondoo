variable "gitlab_token" {
  description = "The GitLab Token"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the GitLab integration
resource "mondoo_integration_gitlab" "gitlab_integration" {
  name = "GitLab Integration"

  # base_url = "" # optional, otherwise gitlab.com
  # group = "" # optional, otherwise all groups are discovered

  # configure discovery options  
  discovery = {
    groups        = true
    projects      = true
    terraform     = true
    k8s_manifests = true
  }

  credentials = {
    token = var.gitlab_token
  }
}
