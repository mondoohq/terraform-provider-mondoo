// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccServiceAccountResource(t *testing.T) {
	orgID, err := getOrgId()
	if err != nil {
		t.Fatal(err)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccServiceAccountOrgResourceConfig(orgID, "one"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_service_account.org", "name", "one"),
				),
			},
			{
				Config: testAccServiceAccountSpaceResourceConfig(orgID, "one"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_service_account.space", "name", "one"),
				),
			},
			// ImportState testing
			// service accounts cannot be imported
			//{
			//	ResourceName:      "mondoo_service_account.test",
			//	ImportState:       true,
			//	ImportStateVerify: true,
			//},
			// Update and Read testing
			{
				Config: testAccServiceAccountOrgResourceConfig(orgID, "two"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_service_account.org", "name", "two"),
				),
			},
			{
				Config: testAccServiceAccountSpaceResourceConfig(orgID, "two"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_service_account.space", "name", "two"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccServiceAccountOrgResourceConfig(resourceOrgID, configurableAttribute string) string {
	return fmt.Sprintf(`
resource "mondoo_service_account" "org" {
  org_id = %[1]q
  name = %[2]q
}
`, resourceOrgID, configurableAttribute)
}

func testAccServiceAccountSpaceResourceConfig(resourceOrgID, configurableAttribute string) string {
	return fmt.Sprintf(`
resource "mondoo_space" "test" {
  org_id = %[1]q
  name = "registration-token-test"
}

resource "mondoo_service_account" "space" {
  space_id = mondoo_space.test.id
  name = %[2]q
}
`, resourceOrgID, configurableAttribute)
}
