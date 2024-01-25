// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPolicyAssignmentResource(t *testing.T) {
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
				Config: testAccPolicyAssignmentResourceConfig(orgID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_policy_assignment.space", "state", "enabled"),
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
				Config: testAccPolicyAssignmentResourceConfig(orgID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_policy_assignment.space", "state", "enabled"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccPolicyAssignmentResourceConfig(resourceOrgID string) string {
	return fmt.Sprintf(`

resource "mondoo_space" "test" {
  org_id = %[1]q
  name = "registration-token-test"
}

resource "mondoo_policy_assignment" "space" {
  space_id = mondoo_space.test.id

  policies = [
    "//policy.api.mondoo.app/policies/mondoo-aws-security",
  ]

  depends_on = [
    mondoo_space.test
  ]
}
`, resourceOrgID)
}
