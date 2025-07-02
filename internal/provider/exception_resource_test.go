// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccExceptionResource(t *testing.T) {
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
				Config: testAccExceptionResourceConfig(orgID, "A justification goes here"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_exception.test", "justification", "A justification goes here"),
					resource.TestCheckResourceAttr("mondoo_exception.test", "action", "RISK_ACCEPTED"),
				),
			},
			// ImportState testing
			// NOTE: not implemented by resource
			//
			// Update and Read testing
			{
				Config: testAccExceptionResourceConfig(orgID, "A more complex justification"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_exception.test", "justification", "A more complex justification"),
					resource.TestCheckResourceAttr("mondoo_exception.test", "action", "RISK_ACCEPTED"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccExceptionResourceConfig(resourceOrgID, justification string) string {
	return fmt.Sprintf(`
resource "mondoo_space" "test" {
  org_id = %[1]q
  name = "registration-token-test"
}

resource "mondoo_exception" "test" {
  scope_mrn  = "//captain.api.mondoo.app/spaces/${mondoo_space.test.id}"
  check_mrns = [
    "//policy.api.mondoo.app/queries/mondoo-linux-security-permissions-on-ssh-private-host-key-files-are-configured"
  ]
  justification = %[2]q
  action        = "RISK_ACCEPTED"
  valid_until   = "2030-12-31"
}
`, resourceOrgID, justification)
}
