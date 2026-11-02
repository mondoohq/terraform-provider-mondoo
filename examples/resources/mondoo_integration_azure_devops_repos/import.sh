# Import using integration MRN. The client secret or certificate is never read
# back: set it in the configuration, and the next apply sends it.
terraform import mondoo_integration_azure_devops_repos.organization "//captain.api.mondoo.app/organizations/your-org-1234567/integrations/2Abd08lk860"
