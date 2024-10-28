// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSlackResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccSlackResourceConfig(accSpace.ID(), "one"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_slack.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_slack.test", "space_id", accSpace.ID()),
				),
			},
			{
				Config: testAccSlackResourceWithSpaceInProviderConfig(accSpace.ID(), "two"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_slack.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_slack.test", "space_id", accSpace.ID()),
				),
			},
			// ImportState testing
			// @afiune this doesn't work since most of our resources doesn't have the `id` attribute
			// if we add it, instead of the `mrn` or as a copy, this import test will work
			// {
			// ResourceName:      "mondoo_integration_slack.test",
			// ImportState:       true,
			// ImportStateVerify: true,
			// },
			// Update and Read testing
			{
				Config: testAccSlackResourceConfig(accSpace.ID(), "three"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_slack.test", "name", "three"),
					resource.TestCheckResourceAttr("mondoo_integration_slack.test", "space_id", accSpace.ID()),
				),
			},
			{
				Config: testAccSlackResourceWithSpaceInProviderConfig(accSpace.ID(), "four"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_slack.test", "name", "four"),
					resource.TestCheckResourceAttr("mondoo_integration_slack.test", "space_id", accSpace.ID()),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccSlackResourceConfig(spaceID, intName string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_slack" "test" {
	space_id = %[1]q
  name = %[2]q
	slack_token = "xoxa-1234567890abc"
}
`, spaceID, intName)
}

func testAccSlackResourceWithSpaceInProviderConfig(spaceID, intName string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_slack" "test" {
  name = %[2]q
	slack_token = "xoxa-1234567890abc"
}
`, spaceID, intName)
}
