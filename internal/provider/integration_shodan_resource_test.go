// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccShodanResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccShodanResourceConfig(accSpace.ID(), "one", []string{"mondoo.com"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "space_id", accSpace.ID()),
				),
			},
			{
				Config: testAccShodanResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "abctoken12345"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "space_id", accSpace.ID()),
				),
			},
			// ImportState testing
			// @afiune this doesn't work since most of our resources doesn't have the `id` attribute
			// if we add it, instead of the `mrn` or as a copy, this import test will work
			// {
			// ResourceName:      "mondoo_integration_shodan.test",
			// ImportState:       true,
			// ImportStateVerify: true,
			// },
			// Update and Read testing
			{
				Config: testAccShodanResourceConfig(accSpace.ID(), "three", []string{"mondoo.com"}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "name", "three"),
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "space_id", accSpace.ID()),
				),
			},
			{
				Config: testAccShodanResourceWithSpaceInProviderConfig(accSpace.ID(), "four", "0987xyzabc7654"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "name", "four"),
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "space_id", accSpace.ID()),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccShodanResourceConfig(spaceID, intName string, targets []string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_shodan" "test" {
	space_id = %[1]q
  name = %[2]q
  targets = %[3]q
	credentials = {
	  token = "abcd1234567890"
	}
}
`, spaceID, intName, targets)
}

func testAccShodanResourceWithSpaceInProviderConfig(spaceID, intName, token string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_shodan" "test" {
  name = %[2]q
  targets = ["8.8.8.8"]
	credentials = {
	  token = %[3]q
	}
}
`, spaceID, intName, token)
}
