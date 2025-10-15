// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestExceptionResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testCreateException(accSpace.ID(), accSpace.MRN(), "RISK_ACCEPTED"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_exception.windows_defender_exception", "action", "RISK_ACCEPTED"),
				),
			},
			// Update testing
			{
				Config: testCreateException(accSpace.ID(), accSpace.MRN(), "FALSE_POSITIVE"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_exception.windows_defender_exception", "action", "FALSE_POSITIVE"),
				),
			},
			// // import testing
			// {
			// 	Config:       importException(accSpace.ID()),
			// 	ResourceName: "mondoo_exception.windows_defender_exception",
			// 	ImportStateIdFunc: func(s *terraform.State) (string, error) {
			// 		return s.RootModule().Resources["mondoo_exception.windows_defender_exception"].Primary.Attributes["exception_id"], nil
			// 	},

			// 	ImportStateVerifyIdentifierAttribute: "exception_id",
			// 	ImportState:                          true,
			// 	ImportStateVerify:                    true,
			// },
		},
	})
}

// func importException(spaceId string) string {
// 	return fmt.Sprintf(`
// provider "mondoo" {
//   space = "%s"
// }
// resource "mondoo_exception" "windows_defender_exception" {
// }
// `, spaceId)
// }

func testCreateException(spaceId string, spaceMrn string, action string) string {
	tomorrow := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	return fmt.Sprintf(`
		resource "mondoo_policy_assignment" "cis_policy_assignment_enabled" {
		space_id = "%s"
		policies = [
			"//policy.api.mondoo.app/policies/cis-microsoft-azure-windows-server-2022-dc-level-1",
			"//policy.api.mondoo.app/policies/cis-microsoft-azure-windows-server-2022-ms-level-1",
			"//policy.api.mondoo.app/policies/mondoo-edr-policy",
		]

		state = "enabled"
		}

		# Set exceptions for Windows policies in the space
		resource "mondoo_exception" "windows_defender_exception" {
		justification = "Windows Defender is disabled. Other EDR is used/configured instead."
		scope_mrn =  "%s"
		action        = "%s"
		valid_until = "%s"
		check_mrns = [
			"//policy.api.mondoo.app/queries/cis-microsoft-azure-windows-server-2022--2.3.1.1",
			"//policy.api.mondoo.app/queries/cis-microsoft-azure-windows-server-2022--2.3.1.3",
		]
		depends_on = [
			mondoo_policy_assignment.cis_policy_assignment_enabled
		]
		}
		`, spaceId, spaceMrn, action, tomorrow)
}
