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
				Config: testAccSentinelOneIntegrationResourceConfig(accSpace.ID(), "one", "host", "account", "secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "host", "host"),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "account", "host"),
				),
			},
			{
				Config: testAccSentinelOneIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "host", "account", "cert"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "host", "host"),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "account", "host"),
				),
			},
			// Update and Read testing
			{
				Config: testAccSentinelOneIntegrationResourceConfig(accSpace.ID(), "three", "new-host", "new-account", "secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "name", "three"),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "host", "new-host"),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "account", "new-host"),
				),
			},
			{
				Config: testAccSentinelOneIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "four", "new-host", "new-account", "cert"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "name", "four"),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "host", "new-host"),
					resource.TestCheckResourceAttr("mondoo_integration_sentinel_one.test", "account", "new-host"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccSentinelOneIntegrationResourceConfig(spaceID, intName, host, account, clientSecret string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_sentinel_one" "test" {
  space_id      = %[1]q
  name          = %[2]q

  host          = %[3]q
  account       = %[4]q
	credentials   = {
		client_secret = %[5]q
	}
}
`, spaceID, intName, host, account, clientSecret)
}

func testAccSentinelOneIntegrationResourceWithSpaceInProviderConfig(spaceID, intName, host, account, certificate string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_sentinel_one" "test" {
  name          = %[2]q

  host          = %[3]q
  account       = %[4]q
	credentials   = {
		certificate = %[5]q
	}
}
`, spaceID, intName, host, account, certificate)
}
