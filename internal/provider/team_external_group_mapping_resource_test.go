// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTeamExternalGroupMappingResource(t *testing.T) {
	// These tests are skipped because the tests are run with an agent that is scoped
	// to a specific organization. Assigning these mappings requires a platform admin
	t.SkipNow()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTeamExternalGroupMappingResourceConfig("test-team", "external-group-1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_team_external_group_mapping.test", "external_id", "external-group-1"),
					resource.TestCheckResourceAttrSet("mondoo_team_external_group_mapping.test", "mrn"),
					resource.TestCheckResourceAttrSet("mondoo_team_external_group_mapping.test", "team_mrn"),
					resource.TestCheckResourceAttrSet("mondoo_team_external_group_mapping.test", "created_at"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "mondoo_team_external_group_mapping.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccTeamExternalGroupMappingResourceReplacement(t *testing.T) {
	// These tests are skipped because the tests are run with an agent that is scoped
	// to a specific organization. Assigning these mappings requires a platform admin
	t.SkipNow()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create initial resource
			{
				Config: testAccTeamExternalGroupMappingResourceConfig("test-team", "external-group-1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_team_external_group_mapping.test", "external_id", "external-group-1"),
					resource.TestCheckResourceAttrSet("mondoo_team_external_group_mapping.test", "mrn"),
				),
			},
			// Change external_id - should force replacement
			{
				Config: testAccTeamExternalGroupMappingResourceConfig("test-team", "external-group-2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_team_external_group_mapping.test", "external_id", "external-group-2"),
					resource.TestCheckResourceAttrSet("mondoo_team_external_group_mapping.test", "mrn"),
				),
			},
			// Change team - should force replacement
			{
				Config: testAccTeamExternalGroupMappingResourceConfig("test-team-2", "external-group-2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_team_external_group_mapping.test", "external_id", "external-group-2"),
					resource.TestCheckResourceAttrSet("mondoo_team_external_group_mapping.test", "mrn"),
				),
			},
		},
	})
}

func testAccTeamExternalGroupMappingResourceConfig(teamName, externalId string) string {
	return fmt.Sprintf(`
resource "mondoo_team" "test" {
  name      = %[1]q
  scope_mrn = %[3]q
}

resource "mondoo_team_external_group_mapping" "test" {
  team_mrn    = mondoo_team.test.mrn
  external_id = %[2]q
}
`, teamName, externalId, accSpace.MRN())
}
