// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrganizationResource(t *testing.T) {
	// These tests are skipped because the tests are run with an agent that is scoped
	// to a specific organization. It does not have the ability to create new organizations.
	t.SkipNow()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccOrganizationResourceConfig("name a", "description a"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_organization.test", "name", "name a"),
					resource.TestCheckResourceAttr("mondoo_organization.test", "description", "description a"),
				),
			},
			// Update and Read testing
			{
				Config: testAccOrganizationResourceConfig("name b", "description b"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_organization.test", "name", "name b"),
					resource.TestCheckResourceAttr("mondoo_organization.test", "description", "description b"),
				),
			},
		},
	})
}

func testAccOrganizationResourceConfig(name, description string) string {
	return fmt.Sprintf(`
resource "mondoo_organization" "test" {
  name = %[1]q
  description = %[2]q
}
`, name, description)
}
