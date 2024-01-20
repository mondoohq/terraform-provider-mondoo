// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCustomPolicyResource(t *testing.T) {
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
				Config: testAccCustomPolicyResourceConfig(orgID, "./testdata/policy_1.mql.yaml"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_custom_policy.my_policy", "crc32c", "65df84fe"),
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
				Config: testAccCustomPolicyResourceConfig(orgID, "./testdata/policy_2.mql.yaml"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_custom_policy.my_policy", "crc32c", "4e19b479"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccCustomPolicyResourceConfig(resourceOrgID, policyPath string) string {
	return fmt.Sprintf(`

resource "mondoo_space" "test" {
  org_id = %[1]q
  name = "registration-token-test"
}

resource "mondoo_custom_policy" "my_policy" {
  space_id = mondoo_space.test.id
  source  =  %[2]q
}
`, resourceOrgID, policyPath)
}
