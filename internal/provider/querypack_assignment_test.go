// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccQueryPackAssignmentResource(t *testing.T) {
	orgID, err := getOrgId()
	if err != nil {
		t.Fatal(err)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccQueryPackAssignmentResourceConfig(orgID, "enabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_querypack_assignment.space", "state", "enabled"),
				),
			},
			// ImportState testing
			// NOTE: not implemented by resource
			//{
			//	ResourceName:      "mondoo_registration_token.test",
			//	ImportState:       false,
			//	ImportStateVerify: false,
			//},
			// Update and Read testing
			{
				Config: testAccQueryPackAssignmentResourceConfig(orgID, "disabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_querypack_assignment.space", "state", "disabled"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccQueryPackAssignmentResourceConfig(resourceOrgID string, state string) string {
	return fmt.Sprintf(`

resource "mondoo_space" "test" {
  org_id = %[1]q
  name = "registration-token-test"
}

resource "mondoo_querypack_assignment" "space" {
  space_id = mondoo_space.test.id

  querypacks = [
    "//policy.api.mondoo.app/policies/mondoo-incident-response-aws",
  ]

  state = %[2]q

  depends_on = [
    mondoo_space.test
  ]
}
`, resourceOrgID, state)
}
