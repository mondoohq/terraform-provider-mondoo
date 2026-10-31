// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const action1Addr = "mondoo_integration_action1.test"

// TestAccAction1IntegrationResource needs a real Action1 API client, because the
// API checks it when the integration is created. It is skipped unless
// MONDOO_TEST_ACTION1_CLIENT_ID, MONDOO_TEST_ACTION1_CLIENT_SECRET,
// MONDOO_TEST_ACTION1_ORGANIZATION_ID and MONDOO_TEST_ACTION1_REGION are set.
func TestAccAction1IntegrationResource(t *testing.T) {
	env := map[string]string{}
	for _, k := range []string{"CLIENT_ID", "CLIENT_SECRET", "ORGANIZATION_ID", "REGION"} {
		v := os.Getenv("MONDOO_TEST_ACTION1_" + k)
		if v == "" {
			t.Skip("MONDOO_TEST_ACTION1_* is not set; skipping the Action1 integration acceptance test")
		}
		env[k] = v
	}

	config := func(name, region, credential string) string {
		return fmt.Sprintf(`
resource "mondoo_integration_action1" "test" {
  space_id        = %q
  name            = %q
  region          = %q
  organization_id = %q
  %s
}
`, accSpace.ID(), name, region, env["ORGANIZATION_ID"], credential)
	}
	inline := fmt.Sprintf("client_id = %q\n  client_secret = %q", env["CLIENT_ID"], env["CLIENT_SECRET"])

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Rejected at plan, before any API call. First, so the post-test destroy runs with a valid config.
			{
				Config:      config("tf-acc action1", "https://app.action1.com", inline),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
			{
				Config:      config("tf-acc action1", env["REGION"], inline+"\n  credential_mrn = \"//some/credential\""),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			{
				Config:      config("tf-acc action1", env["REGION"], fmt.Sprintf("client_id = %q", env["CLIENT_ID"])),
				ExpectError: regexp.MustCompile(`Invalid Attribute Combination`),
			},
			// Create with an inline API client: the resource stores it as a credential.
			{
				Config: config("tf-acc action1", env["REGION"], inline),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(action1Addr, "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr(action1Addr, "region", env["REGION"]),
					resource.TestCheckResourceAttrSet(action1Addr, "mrn"),
					resource.TestCheckResourceAttrSet(action1Addr, "managed_credential_mrn"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Rename in place; the credential stays.
			{
				Config: config("tf-acc action1 renamed", env["REGION"], inline),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(action1Addr, plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.TestCheckResourceAttr(action1Addr, "name", "tf-acc action1 renamed"),
			},
			{
				ResourceName: action1Addr,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return s.RootModule().Resources[action1Addr].Primary.Attributes["mrn"], nil
				},
				ImportStateVerifyIdentifierAttribute: "mrn",
				ImportState:                          true,
				ImportStateVerify:                    true,
				// The API never returns the credential.
				ImportStateVerifyIgnore: []string{"client_id", "client_secret", "managed_credential_mrn"},
			},
		},
	})
}
