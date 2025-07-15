// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccWorkspaceResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccWorkspaceResourceConfig(accSpace.ID(), "my test workspace", "development"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test_with_space_id", "name", "my test workspace"),
					resource.TestCheckResourceAttr("mondoo_workspace.test_with_space_id", "space_id", accSpace.ID()),
				),
			},
			{
				Config: testAccWorkspaceResourceWithSpaceInProviderConfig(accSpace.ID(), "some workspace", "qa"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test_without_space_id", "name", "some workspace"),
					resource.TestCheckResourceAttr("mondoo_workspace.test_without_space_id", "space_id", accSpace.ID()),
				),
			},
			// ImportState testing
			// @afiune this doesn't work since most of our resources doesn't have the `id` attribute
			// if we add it, instead of the `mrn` or as a copy, this import test will work
			// {
			// ResourceName:      "mondoo_workspace.test",
			// ImportState:       true,
			// ImportStateVerify: true,
			// },
			// Update and Read testing
			{
				Config: testAccWorkspaceResourceConfig(accSpace.ID(), "my updated workspace", "production"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test_with_space_id", "name", "my updated workspace"),
					resource.TestCheckResourceAttr("mondoo_workspace.test_with_space_id", "space_id", accSpace.ID()),
				),
			},
			{
				Config: testAccWorkspaceResourceWithSpaceInProviderConfig(accSpace.ID(), "updated workspace", "production"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test_without_space_id", "name", "updated workspace"),
					resource.TestCheckResourceAttr("mondoo_workspace.test_without_space_id", "space_id", accSpace.ID()),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccWorkspaceResourceConfig(spaceID, name, env string) string {
	return fmt.Sprintf(`
resource "mondoo_workspace" "test_with_space_id" {
  space_id         = %[1]q
  name             = %[2]q
  asset_selections = [
    {
      conditions = [
        {
          operator = "AND"
          key_value_condition = {
            field    = "LABELS"
            operator = "CONTAINS"
            values = [
              {
                key   = "environment"
                value = %[3]q
              }
            ]
          }
        }
      ]
    }
  ]
}
`, spaceID, name, env)
}

func testAccWorkspaceResourceWithSpaceInProviderConfig(spaceID, name, env string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}

resource "mondoo_workspace" "test_without_space_id" {
  name             = %[2]q
  asset_selections = [
    {
      conditions = [
        {
          operator = "AND"
          key_value_condition = {
            field    = "LABELS"
            operator = "CONTAINS"
            values = [
              {
                key   = "environment"
                value = %[3]q
              }
            ]
          }
        }
      ]
    }
  ]
}
`, spaceID, name, env)
}
