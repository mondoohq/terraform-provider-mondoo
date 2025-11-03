// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestAccIAMBindingResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testIAMBindingConfig("//captain.api.mondoo.app/teams/testteam", accSpace.MRN(), "//iam.api.mondoo.app/roles/editor"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_iam_binding.test", "identity_mrn", "//captain.api.mondoo.app/teams/testteam"),
					resource.TestCheckResourceAttr("mondoo_iam_binding.test", "resource_mrn", accSpace.MRN()),
					resource.TestCheckResourceAttr("mondoo_iam_binding.test", "roles.#", "1"),
					resource.TestCheckResourceAttr("mondoo_iam_binding.test", "roles.0", "//iam.api.mondoo.app/roles/editor"),
				),
			},
			// Update with same role but referred as the short name (should result in no changes)
			{
				Config: testIAMBindingConfig("//captain.api.mondoo.app/teams/testteam", accSpace.MRN(), "editor"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectEmptyPlan(),
					},
				},
			},
		},
	})
}

func testIAMBindingConfig(identityMrn string, resourceMrn string, role string) string {
	return fmt.Sprintf(`
	resource "mondoo_iam_binding" "test" {
		identity_mrn = "%s"
		resource_mrn = "%s"
		roles        = ["%s"]
	}
	`, identityMrn, resourceMrn, role)
}
