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

  credentials = {
    token = var.github_token
  }
}
