// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRegistrastionTokenResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccRegistrationTokenResourceConfig("one"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_registration_token.test", "configurable_attribute", "one"),
					resource.TestCheckResourceAttr("mondoo_registration_token.test", "defaulted", "example value when not configured"),
					resource.TestCheckResourceAttr("mondoo_registration_token.test", "id", "example-id"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "mondoo_registration_token.test",
				ImportState:       true,
				ImportStateVerify: true,
				// This is not normally necessary, but is here because this
				// example code does not have an actual upstream service.
				// Once the Read method is able to refresh information from
				// the upstream service, this can be removed.
				ImportStateVerifyIgnore: []string{"configurable_attribute", "defaulted"},
			},
			// Update and Read testing
			{
				Config: testAccRegistrationTokenResourceConfig("two"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_registration_token.test", "configurable_attribute", "two"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccRegistrationTokenResourceConfig(configurableAttribute string) string {
	return fmt.Sprintf(`
resource "mondoo_registration_token" "test" {
  configurable_attribute = %[1]q
}
`, configurableAttribute)
}
