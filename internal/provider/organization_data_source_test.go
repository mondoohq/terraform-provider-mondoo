// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrganizationDataSource(t *testing.T) {
	orgID, err := getOrgId()
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccOrganizationDataSourceConfig(orgID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.mondoo_organization.org", "id", orgID),
				),
			},
		},
	})
}

func TestAccOrganizationDataSourceWithAnnotations(t *testing.T) {
	// Skipped: requires ability to create organizations with annotations
	t.SkipNow()
	orgID, err := getOrgId()
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccOrganizationDataSourceConfigWithAnnotations(orgID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.mondoo_organization.org", "id", orgID),
					resource.TestCheckResourceAttr("data.mondoo_organization.org", "annotations.env", "test"),
				),
			},
		},
	})
}

func testAccOrganizationDataSourceConfigWithAnnotations(orgId string) string {
	return fmt.Sprintf(`
resource "mondoo_organization" "test" {
  name = "test-org-tags-%[1]s"

  annotations = {
    env = "test"
  }
}

data "mondoo_organization" "org" {
  id = mondoo_organization.test.id

  depends_on = [
    mondoo_organization.test
  ]
}
`, orgId)
}

func testAccOrganizationDataSourceConfig(orgId string) string {
	return fmt.Sprintf(`
data "mondoo_organization" "org"{
	id = %[1]q
}

output "org_id" {
  value = data.mondoo_organization.org.id
}
`, orgId)
}
