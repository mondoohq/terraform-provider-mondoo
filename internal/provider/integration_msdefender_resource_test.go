// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMsDefenderIntegrationResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccMsDefenderIntegrationResourceConfig(accSpace.ID(), "one", "ffffffff-ffff-ffff-ffff-ffffffffffff", "ffffffff-ffff-ffff-ffff-ffffffffffff", `["ffffffff-ffff-ffff-ffff-ffffffffffff", "ffffffff-ffff-ffff-ffff-ffffffffffff"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "tenant_id", "ffffffff-ffff-ffff-ffff-ffffffffffff"),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "client_id", "ffffffff-ffff-ffff-ffff-ffffffffffff"),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "subscription_allow_list.0", "ffffffff-ffff-ffff-ffff-ffffffffffff"),
				),
			},
			{
				Config: testAccMsDefenderIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "abcd1234567890", `["ffffffff-ffff-ffff-ffff-ffffffffffff", "ffffffff-ffff-ffff-ffff-ffffffffffff"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "subscription_deny_list.0", "ffffffff-ffff-ffff-ffff-ffffffffffff"),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "credentials.pem_file", "abcd1234567890"),
				),
			},
			// Update and Read testing
			{
				Config: testAccMsDefenderIntegrationResourceConfig(accSpace.ID(), "three", "ffffffff-ffff-ffff-ffff-ffffffffff", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", `["aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "name", "three"),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "tenant_id", "ffffffff-ffff-ffff-ffff-ffffffffff"),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "client_id", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "subscription_allow_list.0", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
				),
			},
			{
				Config: testAccMsDefenderIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "four", "abcd1234567890", `["aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "name", "four"),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "subscription_deny_list.0", "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"),
					resource.TestCheckResourceAttr("mondoo_integration_msdefender.msdefender_integration", "credentials.pem_file", "abcd1234567890"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccMsDefenderIntegrationResourceConfig(spaceID, intName, tenantID, clientID, allowList string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_msdefender" "msdefender_integration" {
  space_id  = %[1]q
  name      = %[2]q
  tenant_id = %[3]q
  client_id = %[4]q
  subscription_allow_list= %[5]s
  credentials = {
    pem_file = "abcd1234567890"
  }
}
`, spaceID, intName, tenantID, clientID, allowList)
}

func testAccMsDefenderIntegrationResourceWithSpaceInProviderConfig(spaceID, intName, pemFile, denyList string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_msdefender" "msdefender_integration" {
  name      = %[2]q
  tenant_id = "ffffffff-ffff-ffff-ffff-ffffffffffff"
  client_id = "ffffffff-ffff-ffff-ffff-ffffffffffff"
  subscription_deny_list = %[3]s
  credentials = {
    pem_file = %[4]q
  }
}
`, spaceID, intName, denyList, pemFile)
}
