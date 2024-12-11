// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccZendeskResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccZendeskResourceConfig(accSpace.ID(), "one", "your-subdomain", "zendeskowner@email.com", `[
					{id: "123456", value: "custom_value_1"},
					{id: "123457", value: "custom_value_2"},
				]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "subdomain", "your-subdomain"),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "email", "zendeskowner@email.com"),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "custom_fields.0.value", "custom_value_1"),
				),
			},
			{
				Config: testAccZendeskResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "abctoken12345", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "auto_create", "true"),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "auto_close", "true"),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "credentials.token", "abctoken12345"),
				),
			},
			// Update and Read testing
			{
				Config: testAccZendeskResourceConfig(accSpace.ID(), "three", "updated-subdomain", "updated@email.com", `[
					{id: "123456", value: "new_custom_value_1"},
				]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "name", "three"),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "subdomain", "updated-subdomain"),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "email", "updated@email.com"),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "custom_fields.0.value", "new_custom_value_1"),
				),
			},
			{
				Config: testAccZendeskResourceWithSpaceInProviderConfig(accSpace.ID(), "four", "0987xyzabc7654", false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "name", "four"),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "auto_create", "false"),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "auto_close", "false"),
					resource.TestCheckResourceAttr("mondoo_integration_zendesk.test", "credentials.token", "0987xyzabc7654"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccZendeskResourceConfig(spaceID, name, subdomain, email, customFields string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_zendesk" "test" {
  space_id = %[1]q
  name      = %[2]q
  subdomain = %[3]q
  email     = %[4]q

  custom_fields = %[5]s

  auto_create = true
  auto_close  = true

  credentials = {
    token = "abcd1234567890"
  }
}
`, spaceID, name, subdomain, email, customFields)
}

func testAccZendeskResourceWithSpaceInProviderConfig(spaceID, intName, token string, autoCreate, autoClose bool) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_zendesk" "test" {
  name      = %[2]q
  subdomain = "your-subdomain"
  email     = "zendeskowner@email.com"

  custom_fields = [
    {
      id    = "123456"
      value = "custom_value_1"
    },
    {
      id    = "123457"
      value = "custom_value_2"
    }
  ]

  auto_create = %[3]t
  auto_close  = %[4]t

  credentials = {
    token = %[5]q
  }
}
`, spaceID, intName, autoCreate, autoClose, token)
}
