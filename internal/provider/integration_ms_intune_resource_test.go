// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMsIntuneIntegrationResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccMsIntuneIntegrationResourceConfig(accSpace.ID(), "one", "a-tenant-id", "a-client-id", "a-client-secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "tenant_id", "a-tenant-id"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "client_id", "a-client-id"),
				),
			},
			{
				Config: testAccMsIntuneIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "b-tenant-id", "b-client-id", "b-client-secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "tenant_id", "b-tenant-id"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "client_id", "b-client-id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccMsIntuneIntegrationResourceConfig(accSpace.ID(), "three", "new-tenant-id", "new-client-id", "new-client-secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "name", "three"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "tenant_id", "new-tenant-id"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "client_id", "new-client-id"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccMsIntuneIntegrationResourceConfig(spaceID, intName, tenantID, clientID, clientSecret string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_ms_intune" "test" {
  space_id  = %[1]q
  name      = %[2]q
  tenant_id = %[3]q
  client_id = %[4]q
  credentials = {
    client_secret = %[5]q
  }
}
`, spaceID, intName, tenantID, clientID, clientSecret)
}

func testAccMsIntuneIntegrationResourceWithSpaceInProviderConfig(spaceID, intName, tenantID, clientID, clientSecret string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_ms_intune" "test" {
  name      = %[2]q
  tenant_id = %[3]q
  client_id = %[4]q
  credentials = {
    client_secret = %[5]q
  }
}
`, spaceID, intName, tenantID, clientID, clientSecret)
}
