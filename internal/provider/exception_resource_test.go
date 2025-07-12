package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestExceptionResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testCreateException(accSpace.ID(), accSpace.MRN()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_exception.windows_defender_exception", "action", "RISK_ACCEPTED"),
				),
			},
		},
	})
}

func testCreateException(spaceId string, spaceMrn string) string {
	return fmt.Sprintf(`
		resource "mondoo_policy_assignment" "cis_policy_assignment_enabled" {
		space_id = "%s"
		policies = [
			"//policy.api.mondoo.app/policies/cis-microsoft-windows-10-l1-ce",
			"//policy.api.mondoo.app/policies/cis-microsoft-windows-10-l1-bl",
			"//policy.api.mondoo.app/policies/cis-microsoft-windows-11-l1-ce",
			"//policy.api.mondoo.app/policies/cis-microsoft-windows-11-l1-bl",
			"//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2016-dc-level-1",
			"//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2016-ms-level-1",
			"//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2019-dc-level-1",
			"//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2019-ms-level-1",
			"//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2022-dc-level-1",
			"//policy.api.mondoo.app/policies/cis-microsoft-windows-server-2022-ms-level-1",
			"//policy.api.mondoo.app/policies/cis-microsoft-azure-windows-server-2019-dc-level-1",
			"//policy.api.mondoo.app/policies/cis-microsoft-azure-windows-server-2019-ms-level-1",
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
		action        = "RISK_ACCEPTED"
		valid_until = "2025-09-09"
		check_mrns = [
			"//policy.api.mondoo.app/queries/cis-microsoft-windows-10--18.10.42.5.1",
			"//policy.api.mondoo.app/queries/cis-microsoft-windows-11--18.10.42.5.1",
			"//policy.api.mondoo.app/queries/cis-microsoft-windows-server-2016--18.10.42.5.1",
			"//policy.api.mondoo.app/queries/cis-microsoft-windows-server-2019--18.10.42.5.1",
			"//policy.api.mondoo.app/queries/cis-microsoft-windows-server-2022--18.10.42.5.1",
		]
		depends_on = [
			mondoo_policy_assignment.cis_policy_assignment_enabled
		]
		}
		`, spaceId, spaceMrn)
}
