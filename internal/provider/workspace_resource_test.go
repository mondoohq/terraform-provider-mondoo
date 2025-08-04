// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccWorkspaceResource(t *testing.T) {
	// Generate a random name for the workspace to ensure test isolation
	minV := 1000
	maxV := 3000
	randName := fmt.Sprintf("test-ws-%d", rand.Intn(maxV-minV)+minV)
	randNameUpdated := fmt.Sprintf("%s-updated", randName)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccWorkspaceResourceConfig(accSpace.ID(), randName, "development"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "name", randName),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "space_id", accSpace.ID()),
				),
			},
			{
				Config: testAccWorkspaceResourceConfig(accSpace.ID(), randNameUpdated, "production"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "name", randNameUpdated),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "space_id", accSpace.ID()),
				),
			},
			{
				Config: testAccWorkspaceResourceWithSpaceInProviderConfig(accSpace.ID(), randName, "qa"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "name", randName),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "space_id", accSpace.ID()),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccWorkspaceResourceConfig(spaceID, name, env string) string {
	return fmt.Sprintf(`
resource "mondoo_workspace" "test" {
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

resource "mondoo_workspace" "test" {
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
