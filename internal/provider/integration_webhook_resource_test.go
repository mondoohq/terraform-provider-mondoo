// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccWebhookResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccWebhookResourceConfig(accSpace.ID(), "one", "https://example.com/webhook", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "url", "https://example.com/webhook"),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "auto_create", "true"),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "auto_close", "true"),
				),
			},
			{
				Config: testAccWebhookResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "https://example.com/webhook2", false, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "url", "https://example.com/webhook2"),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "auto_create", "false"),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "auto_close", "true"),
				),
			},
			// Update and Read testing
			{
				Config: testAccWebhookResourceConfig(accSpace.ID(), "one", "https://example.com/webhook", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "url", "https://example.com/webhook"),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "auto_create", "true"),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "auto_close", "true"),
				),
			},
			{
				Config: testAccWebhookResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "https://example.com/webhook2", true, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "url", "https://example.com/webhook2"),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "auto_create", "true"),
					resource.TestCheckResourceAttr("mondoo_integration_webhook.test", "auto_close", "false"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccWebhookResourceConfig(spaceID, intName, url string, autoCreate, autoClose bool) string {
	return fmt.Sprintf(`
resource "mondoo_integration_webhook" "test" {
  space_id = %[1]q
  name     = %[2]q
  url      = %[3]q

  auto_create = %[4]t
  auto_close  = %[5]t
}
`, spaceID, intName, url, autoCreate, autoClose)
}

func testAccWebhookResourceWithSpaceInProviderConfig(spaceID, intName, url string, autoCreate, autoClose bool) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_webhook" "test" {
  name = %[2]q
  url  = %[3]q

  auto_create = %[4]t
  auto_close  = %[5]t
}
`, spaceID, intName, url, autoCreate, autoClose)
}
