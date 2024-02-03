// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSpaceResource(t *testing.T) {
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
				Config: testAccSpaceResourceConfig(orgID, "one"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_space.test", "name", "one"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "mondoo_space.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccSpaceResourceConfig(orgID, "two"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_space.test", "name", "two"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccSpaceResourceConfig(resourceOrgID string, name string) string {
	return fmt.Sprintf(`
resource "mondoo_space" "test" {
  org_id = %[1]q
  name = %[2]q
}
`, resourceOrgID, name)
}

func TestAccSpaceWithIDResource(t *testing.T) {
	orgID, err := getOrgId()
	if err != nil {
		t.Fatal(err)
	}

	min := 1000
	max := 3000
	customSpaceID := "my-custom-space-id" + fmt.Sprint(rand.Intn(max-min)+min)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create Space with custom ID
			{
				Config: testAccSpaceResourceConfigWithID(orgID, customSpaceID, "one"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_space.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_space.test", "id", customSpaceID),
				),
			},
			// Update and Read testing
			{
				Config: testAccSpaceResourceConfigWithID(orgID, customSpaceID, "two"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_space.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_space.test", "id", customSpaceID),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccSpaceResourceConfigWithID(resourceOrgID string, id string, name string) string {
	return fmt.Sprintf(`
resource "mondoo_space" "test" {
  org_id = %[1]q
  id     = %[2]q
  name   = %[3]q
}
`, resourceOrgID, id, name)
}
