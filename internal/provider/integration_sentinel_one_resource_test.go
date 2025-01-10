// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSentinelOneIntegrationResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccSentinelOneIntegrationResourceConfig(accSpace.ID(), "one"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "space_id", accSpace.ID()),
				),
			},
			{
				Config: testAccSentinelOneIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "two"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "space_id", accSpace.ID()),
				),
			},
			// Update and Read testing
			{
				Config: testAccSentinelOneIntegrationResourceConfig(accSpace.ID(), "three"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "name", "three"),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "space_id", accSpace.ID()),
				),
			},
			{
				Config: testAccSentinelOneIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "four"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "name", "four"),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "space_id", accSpace.ID()),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccSentinelOneIntegrationResourceConfig(spaceID, intName string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_sentinel_one" "test" {
  space_id      = %[1]q
  name          = %[2]q
}
`, spaceID, intName)
}

func testAccSentinelOneIntegrationResourceWithSpaceInProviderConfig(spaceID, intName string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_sentinel_one" "test" {
  name          = %[2]q
}
`, spaceID, intName)
}
