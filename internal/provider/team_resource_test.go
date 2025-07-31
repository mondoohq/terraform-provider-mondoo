// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTeamResourceWithCustomId(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing with custom ID
			{
				Config: testAccTeamResourceConfigWithId("custom-team-id", "Custom Team", "Team with custom ID"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_team.test", "id", "custom-team-id"),
					resource.TestCheckResourceAttr("mondoo_team.test", "name", "Custom Team"),
					resource.TestCheckResourceAttr("mondoo_team.test", "description", "Team with custom ID"),
					resource.TestCheckResourceAttrSet("mondoo_team.test", "mrn"),
					resource.TestCheckResourceAttrSet("mondoo_team.test", "scope_mrn"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccTeamResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTeamResourceConfig("team-1", "Team responsible for security policies"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_team.test", "name", "team-1"),
					resource.TestCheckResourceAttr("mondoo_team.test", "description", "Team responsible for security policies"),
					resource.TestCheckResourceAttrSet("mondoo_team.test", "id"),
					resource.TestCheckResourceAttrSet("mondoo_team.test", "mrn"),
					resource.TestCheckResourceAttrSet("mondoo_team.test", "scope_mrn"),
				),
			},
			// Update and Read testing
			{
				Config: testAccTeamResourceConfig("team-1", "Updated team description"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_team.test", "name", "team-1"),
					resource.TestCheckResourceAttr("mondoo_team.test", "description", "Updated team description"),
					resource.TestCheckResourceAttrSet("mondoo_team.test", "id"),
					resource.TestCheckResourceAttrSet("mondoo_team.test", "mrn"),
					resource.TestCheckResourceAttrSet("mondoo_team.test", "scope_mrn"),
				),
			},
			// Test team without description
			{
				Config: testAccTeamResourceConfig("team-1", ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_team.test", "name", "team-1"),
					resource.TestCheckResourceAttr("mondoo_team.test", "description", ""),
					resource.TestCheckResourceAttrSet("mondoo_team.test", "id"),
					resource.TestCheckResourceAttrSet("mondoo_team.test", "mrn"),
					resource.TestCheckResourceAttrSet("mondoo_team.test", "scope_mrn"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccTeamResourceConfig(name, description string) string {
	return fmt.Sprintf(`
resource "mondoo_team" "test" {
  name        = %[1]q
  description = %[2]q
  scope_mrn   = %[3]q
}
`, name, description, accSpace.MRN())
}

func testAccTeamResourceConfigWithId(id, name, description string) string {
	return fmt.Sprintf(`
resource "mondoo_team" "test" {
  id          = %[1]q
  name        = %[2]q
  description = %[3]q
  scope_mrn   = %[4]q
}
`, id, name, description, accSpace.MRN())
}
