terraform {
  required_providers {
    mondoo = {
      source = "mondoohq/mondoo"
    }
  }
}

provider "mondoo" {
}

resource "mondoo_space" "my_space_1" {
  name   = "My Space 1"
  org_id = "your-org-1234567"
}

resource "mondoo_scim_group_mapping" "MondooAdmin" {
  org_id = "your-org-1234567"
  group  = "MondooAdmin"
  mappings = [
    # Give admin group access to the organization
    {
      org_mrn : "//captain.api.mondoo.app/organizations/your-org-1234567",
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
