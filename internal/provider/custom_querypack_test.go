// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCustomQueryPackResource(t *testing.T) {
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
				Config: testAccCustomQueryPackResourceConfig(orgID, "./testdata/querypack_1.mql.yaml"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_custom_querypack.my_querypack", "crc32c", "2322167c"),
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
				Config: testAccCustomQueryPackResourceConfig(orgID, "./testdata/querypack_2.mql.yaml"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_custom_querypack.my_querypack", "crc32c", "c4221534"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccCustomQueryPackResourceConfig(resourceOrgID, policyPath string) string {
	return fmt.Sprintf(`

resource "mondoo_space" "test" {
  org_id = %[1]q
  name = "registration-token-test"
}

resource "mondoo_custom_querypack" "my_querypack" {
  space_id = mondoo_space.test.id
  source  =  %[2]q
}
`, resourceOrgID, policyPath)
}
