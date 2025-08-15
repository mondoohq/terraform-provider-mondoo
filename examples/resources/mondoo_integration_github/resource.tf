variable "github_token" {
  description = "The GitHub Token"
  type        = string
  sensitive   = true
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the GitHub integration
resource "mondoo_integration_github" "gh_integration" {
  name  = "GitHub Integration"
  owner = "lunalectric"

  # define a repository if you want to restrict scan to a single repository
  # repository  = "repo1"

  # alternatively, you can define a list of repositories to allow or deny scanning
  # repository_allow_list= ["repo1", "repo2"]
  # repository_deny_list = ["repo1", "repo2"]

  # configure discovery options
  discovery = {
    terraform     = true
    k8s_manifests = true
  }

  # To rotate credentials or explicitly refresh an unreadable token, uncomment on the next apply:
  # force_replace = true

  credentials = {
    token = var.github_token
  }
}
