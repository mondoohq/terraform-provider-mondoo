// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCrowdstrikeIntegrationResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccCrowdstrikeIntegrationResourceConfig(accSpace.ID(), "one", "a-client-id", "a-client-secret", "us-2", "a-member-cid"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_id", "a-client-id"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_secret", "a-client-secret"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "cloud", "us-2"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "member_cid", "a-member-cid"),
				),
			},
			{
				Config: testAccCrowdstrikeIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "id", "secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_id", "id"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_secret", "secret"),
				),
			},
			// Update and Read testing
			{
				Config: testAccCrowdstrikeIntegrationResourceConfig(accSpace.ID(), "three", "new-id", "new-secret", "us-1", "new-cid"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "name", "three"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_id", "new-id"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_secret", "new-secret"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "cloud", "us-1"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "member_cid", "new-cid"),
				),
			},
			{
				Config: testAccCrowdstrikeIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "four", "abc", "xyz"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "name", "four"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_id", "abc"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_secret", "xyz"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccCrowdstrikeIntegrationResourceConfig(spaceID, intName, clientID, clientSecret, cloud, cid string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_crowdstrike" "test" {
  space_id      = %[1]q
  name          = %[2]q
  client_id     = %[3]q
  client_secret = %[4]q
  cloud         = %[5]q
  member_cid    = %[6]q
}
`, spaceID, intName, clientID, clientSecret, cloud, cid)
}

func testAccCrowdstrikeIntegrationResourceWithSpaceInProviderConfig(spaceID, intName, clientID, clientSecret string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_crowdstrike" "test" {
  name          = %[2]q
  client_id     = %[3]q 
  client_secret = %[4]q
}
`, spaceID, intName, clientID, clientSecret)
}
