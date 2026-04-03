// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccTeamMemberResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccTeamMemberResourceConfig("test-member-team", "alice@example.com"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_team_member.test", "identity", "alice@example.com"),
					resource.TestCheckResourceAttrSet("mondoo_team_member.test", "team_mrn"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "mondoo_team_member.test",
				ImportState:       true,
				ImportStateVerify: true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					rs, ok := s.RootModule().Resources["mondoo_team_member.test"]
					if !ok {
						return "", fmt.Errorf("resource not found: mondoo_team_member.test")
					}
					return rs.Primary.Attributes["team_mrn"] + ":" + rs.Primary.Attributes["identity"], nil
				},
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccTeamMemberResourceConfig(teamName, identity string) string {
	return fmt.Sprintf(`
resource "mondoo_team" "test" {
  name      = %[1]q
  scope_mrn = %[3]q
}

resource "mondoo_team_member" "test" {
  team_mrn = mondoo_team.test.mrn
  identity = %[2]q
}
`, teamName, identity, accSpace.MRN())
}
