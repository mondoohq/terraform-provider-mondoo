// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
)

func TestAccAssetRoutingTableResource(t *testing.T) {
	if os.Getenv("RUN_ASSET_ROUTING_TESTS") != "true" {
		t.Skip("skipping: asset routing tests only run on a single TF version to avoid parallel conflicts")
	}
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
			// Create with a single rule
			{
				Config: testAccAssetRoutingTableConfig(orgMrn, targetSpaceMrn),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_asset_routing_table.test", "org_mrn", orgMrn),
					resource.TestCheckResourceAttr("mondoo_asset_routing_table.test", "rule.#", "1"),
					resource.TestCheckResourceAttrSet("mondoo_asset_routing_table.test", "rule.0.mrn"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_table.test", "rule.0.target_space_mrn", targetSpaceMrn),
					resource.TestCheckResourceAttr("mondoo_asset_routing_table.test", "rule.0.condition.#", "1"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_table.test", "rule.0.condition.0.field", "PLATFORM"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_table.test", "rule.0.condition.0.operator", "EQUAL"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_table.test", "rule.0.condition.0.values.0", "ubuntu"),
				),
			},
			// Update to two rules (add a catch-all)
			{
				Config: testAccAssetRoutingTableConfigWithCatchAll(orgMrn, targetSpaceMrn),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_asset_routing_table.test", "rule.#", "2"),
					resource.TestCheckResourceAttrSet("mondoo_asset_routing_table.test", "rule.0.mrn"),
					resource.TestCheckResourceAttrSet("mondoo_asset_routing_table.test", "rule.1.mrn"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_table.test", "rule.0.condition.#", "1"),
					resource.TestCheckResourceAttr("mondoo_asset_routing_table.test", "rule.1.target_space_mrn", targetSpaceMrn),
					resource.TestCheckResourceAttr("mondoo_asset_routing_table.test", "rule.1.condition.#", "0"),
				),
			},
			// ImportState testing
			{
				ResourceName:                         "mondoo_asset_routing_table.test",
				ImportState:                          true,
				ImportStateId:                        orgMrn,
				ImportStateVerifyIdentifierAttribute: "org_mrn",
				ImportStateVerify:                    true,
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccAssetRoutingTableConfig(orgMrn, targetSpaceMrn string) string {
	return fmt.Sprintf(`
resource "mondoo_asset_routing_table" "test" {
  org_mrn = %[1]q

  rule {
    target_space_mrn = %[2]q

    condition {
      field    = "PLATFORM"
      operator = "EQUAL"
      values   = ["ubuntu"]
    }
  }
}
`, orgMrn, targetSpaceMrn)
}

func testAccAssetRoutingTableConfigWithCatchAll(orgMrn, targetSpaceMrn string) string {
	return fmt.Sprintf(`
resource "mondoo_asset_routing_table" "test" {
  org_mrn = %[1]q

  rule {
    target_space_mrn = %[2]q

    condition {
      field    = "PLATFORM"
      operator = "EQUAL"
      values   = ["ubuntu"]
    }
  }

  rule {
    target_space_mrn = %[2]q
  }
}
`, orgMrn, targetSpaceMrn)
}

func TestTableRulesFromPayload(t *testing.T) {
	rules := tableRulesFromPayload([]AssetRoutingRulePayload{
		{
			Mrn:            "//policy.api.mondoo.app/organizations/org-1/routing-rules/rule-1",
			TargetSpaceMrn: "//captain.api.mondoo.app/spaces/space-1",
			Priority:       0,
			Conditions:     []AssetRoutingConditionPayload{{Field: "LABEL", Operator: "EQUAL", Values: []string{"dev_test"}, Key: "purpose"}},
		},
		{
			Mrn:            "//policy.api.mondoo.app/organizations/org-1/routing-rules/rule-2",
			TargetSpaceMrn: "//captain.api.mondoo.app/spaces/space-2",
			Priority:       1,
		},
	})
	assert.Equal(t, []AssetRoutingTableRuleModel{
		{
			Mrn:            types.StringValue("//policy.api.mondoo.app/organizations/org-1/routing-rules/rule-1"),
			TargetSpaceMrn: types.StringValue("//captain.api.mondoo.app/spaces/space-1"),
			Conditions: []AssetRoutingConditionModel{{
				Field:    types.StringValue("LABEL"),
				Operator: types.StringValue("EQUAL"),
				Values:   ConvertListValue([]string{"dev_test"}),
				Key:      types.StringValue("purpose"),
			}},
		},
		{
			Mrn:            types.StringValue("//policy.api.mondoo.app/organizations/org-1/routing-rules/rule-2"),
			TargetSpaceMrn: types.StringValue("//captain.api.mondoo.app/spaces/space-2"),
			Conditions:     []AssetRoutingConditionModel{},
		},
	}, rules)
}
