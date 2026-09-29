// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// TestIntegrationCrowdstrikeGetConfigurationOptions_Selections checks that
// finding_types and severities are left out of the request when null or
// unknown (the API then keeps what it stores), and that an explicit empty set
// is sent as an empty list (the API then clears it).
func TestIntegrationCrowdstrikeGetConfigurationOptions_Selections(t *testing.T) {
	ctx := context.Background()
	set := func(values ...string) types.Set {
		if values == nil {
			values = []string{} // a nil slice would convert to a null set
		}
		s, diags := types.SetValueFrom(ctx, types.StringType, values)
		require.False(t, diags.HasError())
		return s
	}

	for _, tc := range []struct {
		name             string
		findingTypes     types.Set
		severities       types.Set
		wantFindingTypes *[]mondoov1.CrowdstrikeFalconFindingType
		wantSeverities   *[]mondoov1.CrowdstrikeFalconSeverity
		wantJSON         []string // fragments of the encoded request
		notWantJSON      []string
	}{
		{
			name:         "null",
			findingTypes: types.SetNull(types.StringType),
			severities:   types.SetNull(types.StringType),
			notWantJSON:  []string{`"findingTypes"`, `"severities"`},
		},
		{
			name:         "unknown",
			findingTypes: types.SetUnknown(types.StringType),
			severities:   types.SetUnknown(types.StringType),
			notWantJSON:  []string{`"findingTypes"`, `"severities"`},
		},
		{
			name:             "empty",
			findingTypes:     set(),
			severities:       set(),
			wantFindingTypes: &[]mondoov1.CrowdstrikeFalconFindingType{},
			wantSeverities:   &[]mondoov1.CrowdstrikeFalconSeverity{},
			wantJSON:         []string{`"findingTypes":[]`, `"severities":[]`},
		},
		{
			name:             "values",
			findingTypes:     set("THREAT"),
			severities:       set("CRITICAL", "HIGH"),
			wantFindingTypes: &[]mondoov1.CrowdstrikeFalconFindingType{mondoov1.CrowdstrikeFalconFindingTypeThreat},
			wantSeverities: &[]mondoov1.CrowdstrikeFalconSeverity{
				mondoov1.CrowdstrikeFalconSeverityCritical, mondoov1.CrowdstrikeFalconSeverityHigh,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := integrationCrowdstrikeResourceModel{
				ClientId:     types.StringValue("a-client-id"),
				ClientSecret: types.StringValue("a-client-secret"),
				FindingTypes: tc.findingTypes,
				Severities:   tc.severities,
			}

			opts, diags := m.GetConfigurationOptions(ctx)
			require.False(t, diags.HasError(), diags)
			require.NotNil(t, opts)
			assert.EqualValues(t, "a-client-id", opts.ClientId)
			if tc.wantFindingTypes == nil {
				assert.Nil(t, opts.FindingTypes)
			} else {
				require.NotNil(t, opts.FindingTypes)
				assert.ElementsMatch(t, *tc.wantFindingTypes, *opts.FindingTypes)
			}
			if tc.wantSeverities == nil {
				assert.Nil(t, opts.Severities)
			} else {
				require.NotNil(t, opts.Severities)
				assert.ElementsMatch(t, *tc.wantSeverities, *opts.Severities)
			}

			encoded, err := json.Marshal(opts)
			require.NoError(t, err)
			for _, frag := range tc.wantJSON {
				assert.Contains(t, string(encoded), frag)
			}
			for _, frag := range tc.notWantJSON {
				assert.NotContains(t, string(encoded), frag)
			}
		})
	}
}

// TestIntegrationCrowdstrikeEnumSetValue checks that nothing stored reads back
// as a known empty set, which is what Create records for an omitted attribute.
func TestIntegrationCrowdstrikeEnumSetValue(t *testing.T) {
	ctx := context.Background()
	empty, diags := enumSetValue(ctx, nil)
	require.False(t, diags.HasError())
	assert.False(t, empty.IsNull())
	assert.False(t, empty.IsUnknown())
	assert.Empty(t, empty.Elements())

	values, diags := enumSetValue(ctx, []string{"THREAT", "VULNERABILITY"})
	require.False(t, diags.HasError())
	assert.Len(t, values.Elements(), 2)
}

func TestAccCrowdstrikeIntegrationResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Values outside the API enums fail at plan time
			{
				Config:      testAccCrowdstrikeIntegrationResourceSelectionsConfig(accSpace.ID(), "one", `["ALERTS"]`, `[]`),
				ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Value Match.*finding_types`),
			},
			{
				Config:      testAccCrowdstrikeIntegrationResourceSelectionsConfig(accSpace.ID(), "one", `[]`, `["NONE"]`),
				ExpectError: regexp.MustCompile(`(?s)Invalid Attribute Value Match.*severities`),
			},
			// Create and Read testing
			{
				Config: testAccCrowdstrikeIntegrationResourceConfig(accSpace.ID(), "one", "a-client-id", "a-client-secret", "us-2", "a-member-cid"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_id", "a-client-id"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_secret", "a-client-secret"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "cloud", "us-2"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "member_cid", "a-member-cid"),
					// omitted on create: vulnerabilities only, every severity
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "finding_types.#", "0"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "severities.#", "0"),
				),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: testAccCrowdstrikeIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "id", "secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_id", "id"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_secret", "secret"),
				),
			},
			// Update and Read testing
			{
				Config: testAccCrowdstrikeIntegrationResourceConfig(accSpace.ID(), "three", "new-id", "new-secret", "us-1", "new-cid"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "name", "three"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_id", "new-id"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_secret", "new-secret"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "cloud", "us-1"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "member_cid", "new-cid"),
				),
			},
			// Set finding_types and severities in place
			{
				Config: testAccCrowdstrikeIntegrationResourceSelectionsConfig(accSpace.ID(), "three", `["THREAT"]`, `["CRITICAL", "HIGH"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("mondoo_integration_crowdstrike.test", plancheck.ResourceActionUpdate),
					},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "finding_types.#", "1"),
					resource.TestCheckTypeSetElemAttr("mondoo_integration_crowdstrike.test", "finding_types.*", "THREAT"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "severities.#", "2"),
					resource.TestCheckTypeSetElemAttr("mondoo_integration_crowdstrike.test", "severities.*", "CRITICAL"),
					resource.TestCheckTypeSetElemAttr("mondoo_integration_crowdstrike.test", "severities.*", "HIGH"),
				),
			},
			// Import restores the stored selections (secrets are write-only)
			{
				ResourceName: "mondoo_integration_crowdstrike.test",
				// the resource has no id attribute; import by mrn
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return s.RootModule().Resources["mondoo_integration_crowdstrike.test"].Primary.Attributes["mrn"], nil
				},
				ImportStateVerifyIdentifierAttribute: "mrn",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"client_secret", "member_cid"},
			},
			// Leaving them out keeps what is stored
			{
				Config: testAccCrowdstrikeIntegrationResourceConfig(accSpace.ID(), "three", "new-id", "new-secret", "us-1", "new-cid"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "finding_types.#", "1"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "severities.#", "2"),
				),
			},
			// An explicit empty set clears them
			{
				Config: testAccCrowdstrikeIntegrationResourceSelectionsConfig(accSpace.ID(), "three", `[]`, `[]`),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "finding_types.#", "0"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "severities.#", "0"),
				),
			},
			{
				Config: testAccCrowdstrikeIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "four", "abc", "xyz"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "name", "four"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_id", "abc"),
					resource.TestCheckResourceAttr("mondoo_integration_crowdstrike.test", "client_secret", "xyz"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccCrowdstrikeIntegrationResourceConfig(spaceID, intName, clientID, clientSecret, cloud, cid string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_crowdstrike" "test" {
  space_id      = %[1]q
  name          = %[2]q
  client_id     = %[3]q
  client_secret = %[4]q
  cloud         = %[5]q
  member_cid    = %[6]q
}
`, spaceID, intName, clientID, clientSecret, cloud, cid)
}

func testAccCrowdstrikeIntegrationResourceSelectionsConfig(spaceID, intName, findingTypes, severities string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_crowdstrike" "test" {
  space_id      = %[1]q
  name          = %[2]q
  client_id     = "new-id"
  client_secret = "new-secret"
  cloud         = "us-1"
  member_cid    = "new-cid"
  finding_types = %[3]s
  severities    = %[4]s
}
`, spaceID, intName, findingTypes, severities)
}

func testAccCrowdstrikeIntegrationResourceWithSpaceInProviderConfig(spaceID, intName, clientID, clientSecret string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_crowdstrike" "test" {
  name          = %[2]q
  client_id     = %[3]q 
  client_secret = %[4]q
}
`, spaceID, intName, clientID, clientSecret)
}
