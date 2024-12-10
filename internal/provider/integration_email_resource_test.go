// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEmailIntegrationResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccEmailIntegrationResourceConfig(accSpace.ID(), "one", `[
					{"name": "John Doe", "email": "john@example.com", "is_default": true, "reference_url": "https://example.com"},
					{"name": "Alice Doe", "email": "alice@example.com", "is_default": false, "reference_url": "https://example.com"}
				]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "recipients.0.name", "John Doe"),
				),
			},
			{
				Config: testAccEmailIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "two", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "auto_create", "true"),
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "auto_close", "true"),
				),
			},
			// Update and Read testing
			{
				Config: testAccEmailIntegrationResourceConfig(accSpace.ID(), "three", `[
					{"name": "John Doe", "email": "john.doe@example.com", "is_default": true, "reference_url": "https://newurl.com"},
					{"name": "Alice Doe", "email": "alice.doe@example.com", "is_default": false, "reference_url": "https://newurl.com"}
				]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "name", "three"),
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "recipients.0.reference_url", "https://newurl.com"),
				),
			},
			{
				Config: testAccEmailIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "four", false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "name", "four"),
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "auto_create", "false"),
					resource.TestCheckResourceAttr("mondoo_integration_email.test", "auto_close", "false"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccEmailIntegrationResourceConfig(spaceID, intName, recipients string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_email" "test" {
  space_id = %[1]q
  name = %[2]q
  recipients = %[3]s
  auto_create = true
  auto_close = true
}
`, spaceID, intName, recipients)
}

func testAccEmailIntegrationResourceWithSpaceInProviderConfig(spaceID, intName string, autoCreate, autoClose bool) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_email" "test" {
  name = %[2]q
  recipients = [
    {
      name          = "John Doe"
      email         = "john@example.com"
      is_default    = true
      reference_url = "https://example.com"
    },
    {
      name          = "Alice Doe"
      email         = "alice@example.com"
      is_default    = false
      reference_url = "https://example.com"
    }
  ]
  auto_create = %[3]t
  auto_close  = %[4]t
}
`, spaceID, intName, autoCreate, autoClose)
}
