// Copyright Mondoo, Inc. 2024, 2026
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
				Config: testAccPolicyAssignmentResourceConfig(orgID, "enabled"),
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
				Config: testAccPolicyAssignmentResourceConfig(orgID, "disabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_policy_assignment.space", "state", "disabled"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccPolicyAssignmentResourceConfig(resourceOrgID string, state string) string {
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

  state = %[2]q

  depends_on = [
    mondoo_space.test
  ]
}
`, resourceOrgID, state)
}

func TestAccPolicyAssignmentResourceWithScopeMrn(t *testing.T) {
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
				Config: testAccPolicyAssignmentResourceWithScopeMrnConfig(orgID, "enabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_policy_assignment.scope_mrn", "state", "enabled"),
				),
			},
			// Update and Read testing
			{
				Config: testAccPolicyAssignmentResourceWithScopeMrnConfig(orgID, "disabled"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_policy_assignment.scope_mrn", "state", "disabled"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccPolicyAssignmentResourceWithScopeMrnConfig(orgID string, state string) string {
	return fmt.Sprintf(`
resource "mondoo_space" "scope_mrn_test" {
  org_id = %[1]q
  name   = "scope-mrn-policy-test"
}

resource "mondoo_policy_assignment" "scope_mrn" {
  scope_mrn = mondoo_space.scope_mrn_test.mrn

  policies = [
    "//policy.api.mondoo.app/policies/mondoo-aws-security",
  ]

  state = %[2]q

  depends_on = [
    mondoo_space.scope_mrn_test
  ]
}
`, orgID, state)
}
