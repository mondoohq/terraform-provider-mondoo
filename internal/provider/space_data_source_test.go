// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSpaceDataSource(t *testing.T) {
	orgID, err := getOrgId()
	if err != nil {
		t.Fatal(err)
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Read testing
			{
				Config: testAccSpaceDataSourceConfig(orgID),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.mondoo_space.space", "name", "test-space"),
				),
			},
		},
	})
}

func testAccSpaceDataSourceConfig(orgId string) string {
	return fmt.Sprintf(`

resource "mondoo_space" "test" {
  org_id = %[1]q
  name = "test-space"
}

data "mondoo_space" "space"{
  id = mondoo_space.test.id

  depends_on = [
    mondoo_space.test
  ]
}

output "space_name" {
  value = data.mondoo_space.space.name
}
`, orgId)
}
