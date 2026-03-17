// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAssetRoutingRuleResource(t *testing.T) {
	orgID, err := getOrgId()
	if err != nil {
		t.Skip("skipping: no org-scoped service account available")
	}
	orgMrn := orgPrefix + orgID
	targetSpaceMrn := accSpace.MRN()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create a rule with platform condition
			{
				Config: testAccAssetRoutingRuleConfig(orgMrn, targetSpaceMrn, 10, "PLATFORM", "EQUAL", "ubuntu"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "org_mrn", orgMrn),
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "target_space_mrn", targetSpaceMrn),
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "priority", "10"),
					resource.TestCheckResourceAttrSet("mondoo_asset_routing_rule.test", "mrn"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "condition.#", "1"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "condition.0.field", "PLATFORM"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "condition.0.operator", "EQUAL"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "condition.0.values.0", "ubuntu"),
				),
			},
			// Update priority and condition
			{
				Config: testAccAssetRoutingRuleConfig(orgMrn, targetSpaceMrn, 20, "HOSTNAME", "CONTAINS", "prod"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "priority", "20"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "condition.0.field", "HOSTNAME"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "condition.0.values.0", "prod"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "mondoo_asset_routing_rule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func TestAccAssetRoutingRuleResourceWithLabel(t *testing.T) {
	orgID, err := getOrgId()
	if err != nil {
		t.Skip("skipping: no org-scoped service account available")
	}
	orgMrn := orgPrefix + orgID
	targetSpaceMrn := accSpace.MRN()

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create a rule with label condition
			{
				Config: testAccAssetRoutingRuleLabelConfig(orgMrn, targetSpaceMrn, 10, "env", "production"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "condition.#", "1"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "condition.0.field", "LABEL"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "condition.0.key", "env"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_rule.test", "condition.0.values.0", "production"),
				),
			},
		},
	})
}

func testAccAssetRoutingRuleConfig(orgMrn, targetSpaceMrn string, priority int, field, operator, value string) string {
	return fmt.Sprintf(`
resource "mondoo_asset_routing_rule" "test" {
  org_mrn          = %[1]q
  target_space_mrn = %[2]q
  priority         = %[3]d

  condition {
    field    = %[4]q
    operator = %[5]q
    values   = [%[6]q]
  }
}
`, orgMrn, targetSpaceMrn, priority, field, operator, value)
}

func testAccAssetRoutingRuleLabelConfig(orgMrn, targetSpaceMrn string, priority int, key, value string) string {
	return fmt.Sprintf(`
resource "mondoo_asset_routing_rule" "test" {
  org_mrn          = %[1]q
  target_space_mrn = %[2]q
  priority         = %[3]d

  condition {
    field    = "LABEL"
    operator = "EQUAL"
    key      = %[4]q
    values   = [%[5]q]
  }
}
`, orgMrn, targetSpaceMrn, priority, key, value)
}
