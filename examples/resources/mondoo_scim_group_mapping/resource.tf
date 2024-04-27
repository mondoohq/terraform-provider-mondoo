provider "mondoo" {}

data "mondoo_organization" "org" {
  id = "reverent-ride-275852"
}

resource "mondoo_space" "my_space_1" {
  name   = "My Space 1"
  org_id = data.mondoo_organization.org.id
}

resource "mondoo_scim_group_mapping" "mondoo_admin" {
  org_id = data.mondoo_organization.org.id
  group  = "MondooAdmin"
  mappings = [
    # Give admin group access to the organization
    {
      org_mrn : data.mondoo_organization.org.mrn,
      iam_role : "//iam.api.mondoo.app/roles/editor"
    },
    # Give admin group access to the space 
    {
      space_mrn : mondoo_space.my_space_1.mrn,
      iam_role : "//iam.api.mondoo.app/roles/viewer"
    }
  ]

  depends_on = [
    mondoo_space.my_space_1
  ]
}

output "org_mrn" {
  description = "The MRN of the organization"
  value       = data.mondoo_organization.org.mrn
}