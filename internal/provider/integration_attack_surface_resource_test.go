// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const attackSurfaceAddr = "mondoo_integration_attack_surface.test"

// The integrations are paused with a one-time schedule, so the test never starts a real scan.
func TestAccAttackSurfaceResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Rejected at plan, before any API call. First, so the post-test destroy runs with a valid config.
			{
				Config:      testAccAttackSurfaceResourceConfig(accSpace.ID(), `domains = ["Example.COM"]`),
				ExpectError: regexp.MustCompile(`must be a lowercase domain name`),
			},
			{
				Config:      testAccAttackSurfaceResourceConfig(accSpace.ID(), `schedule = "1h"`),
				ExpectError: regexp.MustCompile(`Schedule Too Short`),
			},
			// Create with defaults; the name falls back to the first domain.
			{
				Config: testAccAttackSurfaceResourceConfig(accSpace.ID(), ""),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(attackSurfaceAddr, "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr(attackSurfaceAddr, "name", "example.com"),
					resource.TestCheckResourceAttr(attackSurfaceAddr, "domains.#", "1"),
					resource.TestCheckResourceAttr(attackSurfaceAddr, "subdomains", "true"),
					resource.TestCheckResourceAttr(attackSurfaceAddr, "cloud_discovery", "false"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Update in place: name, a second domain and two toggles.
			{
				Config: testAccAttackSurfaceResourceConfig(accSpace.ID(), `
  name               = "tf-acc attack surface"
  domains            = ["example.com", "example.org"]
  subdomain_takeover = true
  port_scan          = false`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(attackSurfaceAddr, plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(attackSurfaceAddr, "name", "tf-acc attack surface"),
					resource.TestCheckResourceAttr(attackSurfaceAddr, "domains.#", "2"),
					resource.TestCheckResourceAttr(attackSurfaceAddr, "subdomain_takeover", "true"),
					resource.TestCheckResourceAttr(attackSurfaceAddr, "port_scan", "false"),
				),
			},
			{
				ResourceName: attackSurfaceAddr,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return s.RootModule().Resources[attackSurfaceAddr].Primary.Attributes["mrn"], nil
				},
				ImportStateVerifyIdentifierAttribute: "mrn",
				ImportState:                          true,
				ImportStateVerify:                    true,
			},
		},
	})
}

// extra adds attributes, replacing a default of the same name.
func testAccAttackSurfaceResourceConfig(spaceID, extra string) string {
	defaults := []string{`domains = ["example.com"]`, `schedule = "once"`, `pause_scanning = true`}
	var lines []string
	for _, d := range defaults {
		name := strings.Fields(d)[0]
		if !regexp.MustCompile(`(?m)^\s*` + name + `\s*=`).MatchString(extra) {
			lines = append(lines, d)
		}
	}
	return fmt.Sprintf(`
resource "mondoo_integration_attack_surface" "test" {
  space_id = %q
  %s
  %s
}
`, spaceID, strings.Join(lines, "\n  "), extra)
}
