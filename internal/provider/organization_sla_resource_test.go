// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	tfresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mondoov1 "go.mondoo.com/mondoo-go"
	"go.mondoo.com/mondoo-go/option"
)

// TestAccOrganizationSlaResource sets, changes, imports and clears the SLAs of
// the test organization. The organization's SLAs govern every space in it, so
// the test runs on a single Terraform version in CI, like the asset routing
// tests, to keep parallel runs from overwriting each other.
func TestAccOrganizationSlaResource(t *testing.T) {
	if os.Getenv("RUN_ORG_SLA_TESTS") != "true" {
		t.Skip("skipping: organization SLA tests only run on a single TF version to avoid parallel conflicts")
	}
	orgID, err := getOrgId()
	if err != nil {
		t.Skip("skipping: no org-scoped service account available")
	}
	orgMrn := orgPrefix + orgID
	const addr = "mondoo_organization_sla.test"

	tfresource.Test(t, tfresource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckOrganizationSlaCleared(orgMrn),
		Steps: []tfresource.TestStep{
			// Create, leaving the optional settings at their defaults.
			{
				Config: testAccOrganizationSlaConfig(orgID, [4][2]int{{7, 5}, {14, 10}, {45, 40}, {90, 80}}, ""),
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr(addr, "org_id", orgID),
					tfresource.TestCheckResourceAttr(addr, "org_mrn", orgMrn),
					tfresource.TestCheckResourceAttr(addr, "critical.days_to_resolve", "7"),
					tfresource.TestCheckResourceAttr(addr, "critical.days_before_warning", "5"),
					tfresource.TestCheckResourceAttr(addr, "high.days_to_resolve", "14"),
					tfresource.TestCheckResourceAttr(addr, "medium.days_to_resolve", "45"),
					tfresource.TestCheckResourceAttr(addr, "low.days_before_warning", "80"),
					tfresource.TestCheckResourceAttr(addr, "start_date_config", "CVE_DETECTED"),
					tfresource.TestCheckResourceAttr(addr, "rating_source", "RISK"),
					// A space in the organization now applies the organization's SLAs.
					testAccCheckSpaceSlasGovernedBy(accSpace.MRN(), orgMrn, 7),
				),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Update every value in place.
			{
				Config: testAccOrganizationSlaConfig(orgID, [4][2]int{{10, 8}, {20, 15}, {60, 60}, {120, 0}}, `
  start_date_config = "CVE_PUBLISHED"
  rating_source     = "CVSS"
`),
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr(addr, "critical.days_to_resolve", "10"),
					tfresource.TestCheckResourceAttr(addr, "high.days_before_warning", "15"),
					tfresource.TestCheckResourceAttr(addr, "medium.days_before_warning", "60"),
					tfresource.TestCheckResourceAttr(addr, "low.days_to_resolve", "120"),
					tfresource.TestCheckResourceAttr(addr, "low.days_before_warning", "0"),
					tfresource.TestCheckResourceAttr(addr, "start_date_config", "CVE_PUBLISHED"),
					tfresource.TestCheckResourceAttr(addr, "rating_source", "CVSS"),
					testAccCheckSpaceSlasGovernedBy(accSpace.MRN(), orgMrn, 10),
				),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Add tag-based rules.
			{
				Config: testAccOrganizationSlaConfig(orgID, [4][2]int{{10, 8}, {20, 15}, {60, 60}, {120, 0}}, testAccOrganizationSlaRules+`
  start_date_config = "CVE_PUBLISHED"
  rating_source     = "CVSS"
`),
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckResourceAttr(addr, "rules.#", "2"),
					tfresource.TestCheckResourceAttr(addr, "rules.0.name", "Internet-facing"),
					tfresource.TestCheckResourceAttr(addr, "rules.0.tags.#", "1"),
					tfresource.TestCheckResourceAttr(addr, "rules.0.critical.days_to_resolve", "1"),
					tfresource.TestCheckResourceAttr(addr, "rules.0.critical.days_before_warning", "0"),
					tfresource.TestCheckResourceAttr(addr, "rules.1.name", "Production"),
					tfresource.TestCheckResourceAttr(addr, "rules.1.tags.#", "2"),
					tfresource.TestCheckResourceAttr(addr, "rules.1.low.days_to_resolve", "60"),
					testAccCheckOrganizationSlaRuleNames(orgMrn, "Internet-facing", "Production"),
				),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Import by organization ID.
			{
				ResourceName:                         addr,
				ImportState:                          true,
				ImportStateId:                        orgID,
				ImportStateVerifyIdentifierAttribute: "org_id",
				ImportStateVerify:                    true,
			},
			// Import by organization MRN.
			{
				ResourceName:                         addr,
				ImportState:                          true,
				ImportStateId:                        orgMrn,
				ImportStateVerifyIdentifierAttribute: "org_id",
				ImportStateVerify:                    true,
			},
			// Dropping the optional settings sets them back to their defaults,
			// and dropping the rules removes them.
			{
				Config: testAccOrganizationSlaConfig(orgID, [4][2]int{{10, 8}, {20, 15}, {60, 60}, {120, 0}}, ""),
				Check: tfresource.ComposeAggregateTestCheckFunc(
					tfresource.TestCheckNoResourceAttr(addr, "rules.#"),
					testAccCheckOrganizationSlaRuleNames(orgMrn),
					tfresource.TestCheckResourceAttr(addr, "start_date_config", "CVE_DETECTED"),
					tfresource.TestCheckResourceAttr(addr, "rating_source", "RISK"),
				),
				ConfigPlanChecks: tfresource.ConfigPlanChecks{
					PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
					PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			// Destroy clears the organization's SLAs; CheckDestroy verifies it.
		},
	})
}

// testAccOrganizationSlaConfig renders the resource with days per rating in
// the order critical, high, medium, low, as {days_to_resolve,
// days_before_warning}, plus any extra attributes.
func testAccOrganizationSlaConfig(orgID string, days [4][2]int, extra string) string {
	return fmt.Sprintf(`
resource "mondoo_organization_sla" "test" {
  org_id = %[1]q

  critical = {
    days_to_resolve     = %[2]d
    days_before_warning = %[3]d
  }
  high = {
    days_to_resolve     = %[4]d
    days_before_warning = %[5]d
  }
  medium = {
    days_to_resolve     = %[6]d
    days_before_warning = %[7]d
  }
  low = {
    days_to_resolve     = %[8]d
    days_before_warning = %[9]d
  }
%[10]s}
`, orgID,
		days[0][0], days[0][1],
		days[1][0], days[1][1],
		days[2][0], days[2][1],
		days[3][0], days[3][1],
		extra)
}

// testAccOrganizationSlaRules is a rule on the attack surface tag and a rule
// on two ordinary tags.
const testAccOrganizationSlaRules = `
  rules = [
    {
      name = "Internet-facing"
      tags = [{ key = "mondoo.com/attack-surface", value = "internet" }]
      critical = {
        days_to_resolve     = 1
        days_before_warning = 0
      }
      high = {
        days_to_resolve     = 5
        days_before_warning = 2
      }
      medium = {
        days_to_resolve     = 30
        days_before_warning = 7
      }
      low = {
        days_to_resolve     = 60
        days_before_warning = 14
      }
    },
    {
      name = "Production"
      tags = [
        { key = "env", value = "production" },
        { key = "env", value = "prod" },
      ]
      critical = {
        days_to_resolve     = 3
        days_before_warning = 1
      }
      high = {
        days_to_resolve     = 7
        days_before_warning = 3
      }
      medium = {
        days_to_resolve     = 30
        days_before_warning = 7
      }
      low = {
        days_to_resolve     = 60
        days_before_warning = 14
      }
    },
  ]
`

// testAccCheckOrganizationSlaRuleNames checks the rules the organization
// stores, by name and in order.
func testAccCheckOrganizationSlaRuleNames(orgMrn string, want ...string) tfresource.TestCheckFunc {
	return func(*terraform.State) error {
		client, err := mondooClient()
		if err != nil {
			return err
		}
		c := &ExtendedGqlClient{Client: client}
		model, err := c.GetSecurityModel(context.Background(), orgMrn)
		if err != nil {
			return err
		}
		if model == nil {
			return fmt.Errorf("organization %s has no security model", orgMrn)
		}
		got := []string{}
		for _, rule := range model.Slas.Rules {
			if rule.IsCustom {
				got = append(got, rule.Name)
			}
		}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			return fmt.Errorf("organization SLA rules = %v, want %v", got, want)
		}
		return nil
	}
}

// testSpaceSecurityModel is the security model of a space as the space sees
// it, including where its SLAs come from.
type testSpaceSecurityModel struct {
	Slas                 SLAsPayload `graphql:"slas"`
	SlasSource           string      `graphql:"slasSource"`
	SlasInheritedFromMrn *string     `graphql:"slasInheritedFromMrn"`
	SlasEditable         bool        `graphql:"slasEditable"`
}

func testAccGetSpaceSecurityModel(spaceMrn string) (testSpaceSecurityModel, error) {
	client, err := mondooClient()
	if err != nil {
		return testSpaceSecurityModel{}, err
	}
	var q struct {
		SecurityModel testSpaceSecurityModel `graphql:"securityModel(scopeMrn: $scopeMrn)"`
	}
	err = client.Query(context.Background(), &q, map[string]interface{}{
		"scopeMrn": mondoov1.String(spaceMrn),
	})
	return q.SecurityModel, err
}

// testAccCheckSpaceSlasGovernedBy checks that a space applies its
// organization's SLAs and can't change them.
func testAccCheckSpaceSlasGovernedBy(spaceMrn, orgMrn string, criticalDays int32) tfresource.TestCheckFunc {
	return func(*terraform.State) error {
		model, err := testAccGetSpaceSecurityModel(spaceMrn)
		if err != nil {
			return err
		}
		if model.SlasSource != slaConfigurationSourceOrganization {
			return fmt.Errorf("space SLAs come from %q, want %q", model.SlasSource, slaConfigurationSourceOrganization)
		}
		if model.SlasInheritedFromMrn == nil || *model.SlasInheritedFromMrn != orgMrn {
			return fmt.Errorf("space SLAs inherited from %v, want %q", model.SlasInheritedFromMrn, orgMrn)
		}
		if model.SlasEditable {
			return fmt.Errorf("space SLAs are editable while the organization sets them")
		}
		for _, f := range model.Slas.Findings {
			if f.Rating == string(mondoov1.ScoreRatingCritical) && f.DaysToResolve != criticalDays {
				return fmt.Errorf("space critical days_to_resolve = %d, want %d", f.DaysToResolve, criticalDays)
			}
		}
		return nil
	}
}

// testAccCheckOrganizationSlaCleared checks that destroy removed the
// organization's SLAs and handed the test space its own SLAs back.
func testAccCheckOrganizationSlaCleared(orgMrn string) tfresource.TestCheckFunc {
	return func(*terraform.State) error {
		client, err := mondooClient()
		if err != nil {
			return err
		}
		c := &ExtendedGqlClient{Client: client}
		model, err := c.GetSecurityModel(context.Background(), orgMrn)
		if err != nil {
			return err
		}
		if model != nil && model.SlasSource == slaConfigurationSourceOrganization {
			return fmt.Errorf("organization %s still sets SLAs after destroy", orgMrn)
		}

		space, err := testAccGetSpaceSecurityModel(accSpace.MRN())
		if err != nil {
			return err
		}
		if space.SlasSource == slaConfigurationSourceOrganization || !space.SlasEditable {
			return fmt.Errorf("space SLAs still governed after destroy: source %q, editable %t", space.SlasSource, space.SlasEditable)
		}
		return nil
	}
}

// organizationSlaSchema returns the resource's schema.
func organizationSlaSchema(t *testing.T) resource.SchemaResponse {
	t.Helper()
	var resp resource.SchemaResponse
	(&OrganizationSlaResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "schema diagnostics: %v", resp.Diagnostics)
	return resp
}

func TestOrganizationSlaSchemaIsValid(t *testing.T) {
	resp := organizationSlaSchema(t)
	diags := resp.Schema.ValidateImplementation(context.Background())
	assert.False(t, diags.HasError(), "schema implementation diagnostics: %v", diags)
}

// organizationSlaTestConfig builds a raw configuration for the resource, as
// Terraform sends it to ValidateConfig. Ratings not given use 30/23. Each
// rule is the four ratings of a rule tagged env = production.
func organizationSlaTestConfig(t *testing.T, ratings map[string]tftypes.Value, rules ...map[string]tftypes.Value) tfsdk.Config {
	t.Helper()
	ctx := context.Background()
	s := organizationSlaSchema(t).Schema
	objectType, ok := s.Type().TerraformType(ctx).(tftypes.Object)
	require.True(t, ok)

	values := map[string]tftypes.Value{
		"org_id":            tftypes.NewValue(tftypes.String, "my-org-123456"),
		"org_mrn":           tftypes.NewValue(tftypes.String, nil),
		"start_date_config": tftypes.NewValue(tftypes.String, nil),
		"rating_source":     tftypes.NewValue(tftypes.String, nil),
	}
	for _, r := range organizationSlaRatings {
		v, ok := ratings[r.attribute]
		if !ok {
			v = organizationSlaTestRating(30, 23)
		}
		values[r.attribute] = v
	}

	rulesType, ok := objectType.AttributeTypes["rules"].(tftypes.List)
	require.True(t, ok)
	ruleType, ok := rulesType.ElementType.(tftypes.Object)
	require.True(t, ok)
	tagsType := ruleType.AttributeTypes["tags"]
	tagType := tagsType.(tftypes.Set).ElementType
	var ruleValues []tftypes.Value
	for i, ruleRatings := range rules {
		rule := map[string]tftypes.Value{
			"name": tftypes.NewValue(tftypes.String, fmt.Sprintf("rule %d", i+1)),
			"tags": tftypes.NewValue(tagsType, []tftypes.Value{
				tftypes.NewValue(tagType, map[string]tftypes.Value{
					"key":   tftypes.NewValue(tftypes.String, "env"),
					"value": tftypes.NewValue(tftypes.String, "production"),
				}),
			}),
		}
		for _, r := range organizationSlaRatings {
			v, ok := ruleRatings[r.attribute]
			if !ok {
				v = organizationSlaTestRating(30, 23)
			}
			rule[r.attribute] = v
		}
		ruleValues = append(ruleValues, tftypes.NewValue(ruleType, rule))
	}
	if ruleValues == nil {
		values["rules"] = tftypes.NewValue(rulesType, nil)
	} else {
		values["rules"] = tftypes.NewValue(rulesType, ruleValues)
	}
	return tfsdk.Config{Schema: s, Raw: tftypes.NewValue(objectType, values)}
}

var organizationSlaTestRatingType = tftypes.Object{AttributeTypes: map[string]tftypes.Type{
	"days_to_resolve":     tftypes.Number,
	"days_before_warning": tftypes.Number,
}}

// organizationSlaTestRating builds a rating object. Each value is an int, nil
// for null, or tftypes.UnknownValue.
func organizationSlaTestRating(daysToResolve, daysBeforeWarning interface{}) tftypes.Value {
	return tftypes.NewValue(organizationSlaTestRatingType, map[string]tftypes.Value{
		"days_to_resolve":     tftypes.NewValue(tftypes.Number, daysToResolve),
		"days_before_warning": tftypes.NewValue(tftypes.Number, daysBeforeWarning),
	})
}

func TestOrganizationSlaValidateConfig(t *testing.T) {
	cases := []struct {
		name    string
		ratings map[string]tftypes.Value
		rules   []map[string]tftypes.Value
		wantErr bool
	}{
		{
			name:  "a rule with defaults-like values is valid",
			rules: []map[string]tftypes.Value{{}},
		},
		{
			name: "a rule's warning after its deadline is an error",
			rules: []map[string]tftypes.Value{
				{},
				{"high": organizationSlaTestRating(5, 6)},
			},
			wantErr: true,
		},
		{
			name:  "a rule's unknown days are skipped",
			rules: []map[string]tftypes.Value{{"low": organizationSlaTestRating(tftypes.UnknownValue, 6)}},
		},
		{
			name: "defaults-like values are valid",
		},
		{
			name:    "warning equal to deadline is valid",
			ratings: map[string]tftypes.Value{"high": organizationSlaTestRating(10, 10)},
		},
		{
			name:    "zero days are valid",
			ratings: map[string]tftypes.Value{"low": organizationSlaTestRating(0, 0)},
		},
		{
			name:    "warning after deadline is an error",
			ratings: map[string]tftypes.Value{"medium": organizationSlaTestRating(10, 11)},
			wantErr: true,
		},
		{
			name:    "unknown days_to_resolve is skipped",
			ratings: map[string]tftypes.Value{"critical": organizationSlaTestRating(tftypes.UnknownValue, 11)},
		},
		{
			name:    "unknown days_before_warning is skipped",
			ratings: map[string]tftypes.Value{"critical": organizationSlaTestRating(10, tftypes.UnknownValue)},
		},
		{
			name:    "unknown rating object is skipped",
			ratings: map[string]tftypes.Value{"critical": tftypes.NewValue(organizationSlaTestRatingType, tftypes.UnknownValue)},
		},
		{
			name:    "null rating object is left to the Required check",
			ratings: map[string]tftypes.Value{"critical": tftypes.NewValue(organizationSlaTestRatingType, nil)},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := resource.ValidateConfigRequest{Config: organizationSlaTestConfig(t, tc.ratings, tc.rules...)}
			var resp resource.ValidateConfigResponse
			(&OrganizationSlaResource{}).ValidateConfig(context.Background(), req, &resp)
			assert.Equal(t, tc.wantErr, resp.Diagnostics.HasError(), "diagnostics: %v", resp.Diagnostics)
		})
	}
}

func organizationSlaTestObject(t *testing.T, daysToResolve, daysBeforeWarning int32) types.Object {
	t.Helper()
	obj, diags := types.ObjectValue(organizationSlaRatingAttrTypes, map[string]attr.Value{
		"days_to_resolve":     types.Int32Value(daysToResolve),
		"days_before_warning": types.Int32Value(daysBeforeWarning),
	})
	require.False(t, diags.HasError(), "diagnostics: %v", diags)
	return obj
}

func TestExpandOrganizationSLAsStartsAtTheFirstDetectionInTheSpace(t *testing.T) {
	data := OrganizationSlaResourceModel{
		OrgID:           types.StringValue("my-org-123456"),
		Critical:        organizationSlaTestObject(t, 7, 5),
		High:            organizationSlaTestObject(t, 14, 10),
		Medium:          organizationSlaTestObject(t, 45, 40),
		Low:             organizationSlaTestObject(t, 90, 0),
		StartDateConfig: types.StringValue("FIRST_CVE_DETECTED_IN_SPACE"),
		RatingSource:    types.StringValue("RISK"),
	}

	slas, diags := expandOrganizationSLAs(context.Background(), &data)
	require.False(t, diags.HasError(), "diagnostics: %v", diags)

	require.NotNil(t, slas.StartDateConfig)
	assert.Equal(t, mondoov1.SLAStartDateConfigFirstCveDetectedInSpace, *slas.StartDateConfig)
}

func TestExpandOrganizationSLAs(t *testing.T) {
	data := OrganizationSlaResourceModel{
		OrgID:           types.StringValue("my-org-123456"),
		Critical:        organizationSlaTestObject(t, 7, 5),
		High:            organizationSlaTestObject(t, 14, 10),
		Medium:          organizationSlaTestObject(t, 45, 40),
		Low:             organizationSlaTestObject(t, 90, 0),
		StartDateConfig: types.StringValue("CVE_PUBLISHED"),
		RatingSource:    types.StringValue("CVSS"),
	}

	slas, diags := expandOrganizationSLAs(context.Background(), &data)
	require.False(t, diags.HasError(), "diagnostics: %v", diags)

	// Every rating is always sent, so the API's merge replaces all of them.
	assert.Equal(t, []mondoov1.UpdateFindingsSLAInput{
		{Rating: mondoov1.ScoreRatingCritical, DaysToResolve: 7, DaysBeforeWarning: 5},
		{Rating: mondoov1.ScoreRatingHigh, DaysToResolve: 14, DaysBeforeWarning: 10},
		{Rating: mondoov1.ScoreRatingMedium, DaysToResolve: 45, DaysBeforeWarning: 40},
		{Rating: mondoov1.ScoreRatingLow, DaysToResolve: 90, DaysBeforeWarning: 0},
	}, slas.Findings)
	require.NotNil(t, slas.StartDateConfig)
	assert.Equal(t, mondoov1.SLAStartDateConfigCvePublished, *slas.StartDateConfig)
	require.NotNil(t, slas.RatingSource)
	assert.Equal(t, mondoov1.SLARatingSourceCvss, *slas.RatingSource)
	// No rules are sent as an empty list, which removes the stored ones.
	require.NotNil(t, slas.Rules)
	assert.Empty(t, *slas.Rules)
}

// organizationSlaTestRule builds a rule with the given tags, as key and value
// pairs, and the deadlines 1/0, 5/2, 30/7 and 60/14.
func organizationSlaTestRule(t *testing.T, name string, tags ...[2]string) attr.Value {
	t.Helper()
	tagType := types.ObjectType{AttrTypes: organizationSlaTagAttrTypes}
	tagValues := make([]attr.Value, 0, len(tags))
	for _, tag := range tags {
		obj, diags := types.ObjectValue(organizationSlaTagAttrTypes, map[string]attr.Value{
			"key":   types.StringValue(tag[0]),
			"value": types.StringValue(tag[1]),
		})
		require.False(t, diags.HasError(), "diagnostics: %v", diags)
		tagValues = append(tagValues, obj)
	}
	tagSet, diags := types.SetValue(tagType, tagValues)
	require.False(t, diags.HasError(), "diagnostics: %v", diags)

	rule, diags := types.ObjectValue(organizationSlaRuleAttrTypes, map[string]attr.Value{
		"name":     types.StringValue(name),
		"tags":     tagSet,
		"critical": organizationSlaTestObject(t, 1, 0),
		"high":     organizationSlaTestObject(t, 5, 2),
		"medium":   organizationSlaTestObject(t, 30, 7),
		"low":      organizationSlaTestObject(t, 60, 14),
	})
	require.False(t, diags.HasError(), "diagnostics: %v", diags)
	return rule
}

func organizationSlaTestRules(t *testing.T, rules ...attr.Value) types.List {
	t.Helper()
	list, diags := types.ListValue(organizationSlaRuleObjectType, rules)
	require.False(t, diags.HasError(), "diagnostics: %v", diags)
	return list
}

// organizationSlaTestRuleDeadlines are the deadlines of organizationSlaTestRule
// as the API reports them.
var organizationSlaTestRuleDeadlines = []FindingsSLAPayload{
	{Rating: "CRITICAL", DaysToResolve: 1, DaysBeforeWarning: 0},
	{Rating: "HIGH", DaysToResolve: 5, DaysBeforeWarning: 2},
	{Rating: "MEDIUM", DaysToResolve: 30, DaysBeforeWarning: 7},
	{Rating: "LOW", DaysToResolve: 60, DaysBeforeWarning: 14},
}

func organizationSlaTestRulePayload(name string, tags ...SLATagPayload) SLARulePayload {
	rule := SLARulePayload{Name: name, IsCustom: true, Deadlines: organizationSlaTestRuleDeadlines}
	rule.MatchStrategy.TagMatch.Tags = tags
	return rule
}

func TestExpandOrganizationSLAsRules(t *testing.T) {
	data := OrganizationSlaResourceModel{
		Critical: organizationSlaTestObject(t, 30, 23),
		High:     organizationSlaTestObject(t, 30, 23),
		Medium:   organizationSlaTestObject(t, 60, 53),
		Low:      organizationSlaTestObject(t, 90, 83),
		Rules: organizationSlaTestRules(t,
			organizationSlaTestRule(t, "Internet-facing", [2]string{"mondoo.com/attack-surface", "internet"}),
			organizationSlaTestRule(t, "Production", [2]string{"env", "production"}),
		),
	}

	slas, diags := expandOrganizationSLAs(context.Background(), &data)
	require.False(t, diags.HasError(), "diagnostics: %v", diags)

	deadlines := []mondoov1.UpdateFindingsSLAInput{
		{Rating: mondoov1.ScoreRatingCritical, DaysToResolve: 1, DaysBeforeWarning: 0},
		{Rating: mondoov1.ScoreRatingHigh, DaysToResolve: 5, DaysBeforeWarning: 2},
		{Rating: mondoov1.ScoreRatingMedium, DaysToResolve: 30, DaysBeforeWarning: 7},
		{Rating: mondoov1.ScoreRatingLow, DaysToResolve: 60, DaysBeforeWarning: 14},
	}
	require.NotNil(t, slas.Rules)
	assert.Equal(t, []mondoov1.SLARuleInput{
		{
			Name:          "Internet-facing",
			MatchStrategy: mondoov1.SLAMatchStrategyInput{Tags: &[]mondoov1.SLATagInput{{Key: "mondoo.com/attack-surface", Value: "internet"}}},
			Deadlines:     deadlines,
		},
		{
			Name:          "Production",
			MatchStrategy: mondoov1.SLAMatchStrategyInput{Tags: &[]mondoov1.SLATagInput{{Key: "env", Value: "production"}}},
			Deadlines:     deadlines,
		},
	}, *slas.Rules)
}

func TestExpandOrganizationSLAsAlwaysSendsSettings(t *testing.T) {
	data := OrganizationSlaResourceModel{
		OrgID:           types.StringValue("my-org-123456"),
		Critical:        organizationSlaTestObject(t, 30, 23),
		High:            organizationSlaTestObject(t, 30, 23),
		Medium:          organizationSlaTestObject(t, 60, 53),
		Low:             organizationSlaTestObject(t, 90, 83),
		StartDateConfig: types.StringNull(),
		RatingSource:    types.StringNull(),
	}

	slas, diags := expandOrganizationSLAs(context.Background(), &data)
	require.False(t, diags.HasError(), "diagnostics: %v", diags)

	// Omitting a setting would keep the stored value; the defaults are sent instead.
	require.NotNil(t, slas.StartDateConfig)
	assert.Equal(t, mondoov1.SLAStartDateConfigCveDetected, *slas.StartDateConfig)
	require.NotNil(t, slas.RatingSource)
	assert.Equal(t, mondoov1.SLARatingSourceRisk, *slas.RatingSource)
}

func TestExpandOrganizationSLAsRejectsMissingRating(t *testing.T) {
	data := OrganizationSlaResourceModel{
		OrgID:    types.StringValue("my-org-123456"),
		Critical: organizationSlaTestObject(t, 30, 23),
		High:     types.ObjectNull(organizationSlaRatingAttrTypes),
		Medium:   organizationSlaTestObject(t, 60, 53),
		Low:      organizationSlaTestObject(t, 90, 83),
	}

	_, diags := expandOrganizationSLAs(context.Background(), &data)
	assert.True(t, diags.HasError())
}

func TestFlattenOrganizationSLAs(t *testing.T) {
	// The API's order does not matter: each SLA lands on its own rating.
	payload := SLAsPayload{
		Findings: []FindingsSLAPayload{
			{Rating: "LOW", DaysToResolve: 90, DaysBeforeWarning: 0},
			{Rating: "CRITICAL", DaysToResolve: 7, DaysBeforeWarning: 5},
			{Rating: "MEDIUM", DaysToResolve: 45, DaysBeforeWarning: 40},
			{Rating: "HIGH", DaysToResolve: 14, DaysBeforeWarning: 10},
		},
		StartDateConfig: "CVE_PUBLISHED",
		RatingSource:    "CVSS",
	}

	var data OrganizationSlaResourceModel
	diags := flattenOrganizationSLAs(payload, &data)
	require.False(t, diags.HasError(), "diagnostics: %v", diags)

	assert.Equal(t, organizationSlaTestObject(t, 7, 5), data.Critical)
	assert.Equal(t, organizationSlaTestObject(t, 14, 10), data.High)
	assert.Equal(t, organizationSlaTestObject(t, 45, 40), data.Medium)
	assert.Equal(t, organizationSlaTestObject(t, 90, 0), data.Low)
	assert.Equal(t, types.StringValue("CVE_PUBLISHED"), data.StartDateConfig)
	assert.Equal(t, types.StringValue("CVSS"), data.RatingSource)
}

func TestFlattenOrganizationSLAsRules(t *testing.T) {
	findings := []FindingsSLAPayload{
		{Rating: "CRITICAL", DaysToResolve: 30, DaysBeforeWarning: 23},
		{Rating: "HIGH", DaysToResolve: 30, DaysBeforeWarning: 23},
		{Rating: "MEDIUM", DaysToResolve: 60, DaysBeforeWarning: 53},
		{Rating: "LOW", DaysToResolve: 90, DaysBeforeWarning: 83},
	}

	t.Run("custom rules land in the list, in order", func(t *testing.T) {
		// A server that lists the default rule marks it as not custom.
		defaultRule := SLARulePayload{Name: "Default", Deadlines: findings}
		payload := SLAsPayload{Findings: findings, Rules: []SLARulePayload{
			organizationSlaTestRulePayload("Internet-facing", SLATagPayload{Key: "mondoo.com/attack-surface", Value: "internet"}),
			organizationSlaTestRulePayload("Production", SLATagPayload{Key: "env", Value: "production"}, SLATagPayload{Key: "env", Value: "prod"}),
			defaultRule,
		}}

		var data OrganizationSlaResourceModel
		diags := flattenOrganizationSLAs(payload, &data)
		require.False(t, diags.HasError(), "diagnostics: %v", diags)
		assert.Equal(t, organizationSlaTestRules(t,
			organizationSlaTestRule(t, "Internet-facing", [2]string{"mondoo.com/attack-surface", "internet"}),
			organizationSlaTestRule(t, "Production", [2]string{"env", "production"}, [2]string{"env", "prod"}),
		), data.Rules)
	})

	t.Run("no rules read as null", func(t *testing.T) {
		data := OrganizationSlaResourceModel{
			Rules: organizationSlaTestRules(t, organizationSlaTestRule(t, "Production", [2]string{"env", "production"})),
		}
		diags := flattenOrganizationSLAs(SLAsPayload{Findings: findings}, &data)
		require.False(t, diags.HasError(), "diagnostics: %v", diags)
		assert.True(t, data.Rules.IsNull())
	})

	t.Run("no rules keep a configured empty list", func(t *testing.T) {
		data := OrganizationSlaResourceModel{Rules: organizationSlaTestRules(t)}
		diags := flattenOrganizationSLAs(SLAsPayload{Findings: findings}, &data)
		require.False(t, diags.HasError(), "diagnostics: %v", diags)
		assert.False(t, data.Rules.IsNull())
		assert.Empty(t, data.Rules.Elements())
	})

	t.Run("a rule without a rating is an error", func(t *testing.T) {
		rule := organizationSlaTestRulePayload("Production", SLATagPayload{Key: "env", Value: "production"})
		rule.Deadlines = rule.Deadlines[:3]
		var data OrganizationSlaResourceModel
		diags := flattenOrganizationSLAs(SLAsPayload{Findings: findings, Rules: []SLARulePayload{rule}}, &data)
		assert.True(t, diags.HasError())
	})
}

func TestFlattenOrganizationSLAsDefaultsNullSettings(t *testing.T) {
	payload := SLAsPayload{
		Findings: []FindingsSLAPayload{
			{Rating: "CRITICAL", DaysToResolve: 30, DaysBeforeWarning: 23},
			{Rating: "HIGH", DaysToResolve: 30, DaysBeforeWarning: 23},
			{Rating: "MEDIUM", DaysToResolve: 60, DaysBeforeWarning: 53},
			{Rating: "LOW", DaysToResolve: 90, DaysBeforeWarning: 83},
		},
	}

	var data OrganizationSlaResourceModel
	diags := flattenOrganizationSLAs(payload, &data)
	require.False(t, diags.HasError(), "diagnostics: %v", diags)
	assert.Equal(t, types.StringValue("CVE_DETECTED"), data.StartDateConfig)
	assert.Equal(t, types.StringValue("RISK"), data.RatingSource)
}

func TestFlattenOrganizationSLAsRejectsMissingRating(t *testing.T) {
	payload := SLAsPayload{
		Findings: []FindingsSLAPayload{
			{Rating: "CRITICAL", DaysToResolve: 30, DaysBeforeWarning: 23},
			{Rating: "HIGH", DaysToResolve: 30, DaysBeforeWarning: 23},
			{Rating: "MEDIUM", DaysToResolve: 60, DaysBeforeWarning: 53},
		},
		StartDateConfig: "CVE_DETECTED",
		RatingSource:    "RISK",
	}

	var data OrganizationSlaResourceModel
	diags := flattenOrganizationSLAs(payload, &data)
	assert.True(t, diags.HasError())
}

func TestExpandFlattenOrganizationSLAsRoundTrip(t *testing.T) {
	want := OrganizationSlaResourceModel{
		Critical:        organizationSlaTestObject(t, 1, 0),
		High:            organizationSlaTestObject(t, 2, 1),
		Medium:          organizationSlaTestObject(t, 3, 2),
		Low:             organizationSlaTestObject(t, 4, 4),
		StartDateConfig: types.StringValue("CVE_DETECTED"),
		RatingSource:    types.StringValue("CVSS"),
		Rules: organizationSlaTestRules(t,
			organizationSlaTestRule(t, "Production", [2]string{"env", "production"}, [2]string{"env", "prod"}),
		),
	}

	slas, diags := expandOrganizationSLAs(context.Background(), &want)
	require.False(t, diags.HasError(), "diagnostics: %v", diags)

	// Echo the input back the way the API reports it.
	payload := SLAsPayload{
		StartDateConfig: string(*slas.StartDateConfig),
		RatingSource:    string(*slas.RatingSource),
	}
	for _, f := range slas.Findings {
		payload.Findings = append(payload.Findings, FindingsSLAPayload{
			Rating:            string(f.Rating),
			DaysToResolve:     int32(f.DaysToResolve),
			DaysBeforeWarning: int32(f.DaysBeforeWarning),
		})
	}
	for _, r := range *slas.Rules {
		rule := SLARulePayload{Name: string(r.Name), IsCustom: true}
		for _, tag := range *r.MatchStrategy.Tags {
			rule.MatchStrategy.TagMatch.Tags = append(rule.MatchStrategy.TagMatch.Tags, SLATagPayload{Key: string(tag.Key), Value: string(tag.Value)})
		}
		for _, f := range r.Deadlines {
			rule.Deadlines = append(rule.Deadlines, FindingsSLAPayload{
				Rating:            string(f.Rating),
				DaysToResolve:     int32(f.DaysToResolve),
				DaysBeforeWarning: int32(f.DaysBeforeWarning),
			})
		}
		payload.Rules = append(payload.Rules, rule)
	}

	var got OrganizationSlaResourceModel
	diags = flattenOrganizationSLAs(payload, &got)
	require.False(t, diags.HasError(), "diagnostics: %v", diags)
	assert.Equal(t, want, got)
}

func TestOrganizationIDFromImportID(t *testing.T) {
	cases := []struct {
		importID string
		want     string
		wantErr  bool
	}{
		{importID: "my-org-123456", want: "my-org-123456"},
		{importID: "//captain.api.mondoo.app/organizations/my-org-123456", want: "my-org-123456"},
		{importID: "", wantErr: true},
		{importID: "//captain.api.mondoo.app/organizations/", wantErr: true},
		{importID: "//captain.api.mondoo.app/spaces/my-space-123456", wantErr: true},
		{importID: "//captain.api.mondoo.app/organizations/my-org-123456/spaces/x", wantErr: true},
		{importID: "my org", wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.importID, func(t *testing.T) {
			got, err := organizationIDFromImportID(tc.importID)
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// graphqlRequest is a GraphQL request as the client sends it.
type graphqlRequest struct {
	Query     string                     `json:"query"`
	Variables map[string]json.RawMessage `json:"variables"`
}

// newGraphqlTestClient returns a client whose requests go to a test server
// that records each request and answers with response.
func newGraphqlTestClient(t *testing.T, response string) (*ExtendedGqlClient, *graphqlRequest) {
	t.Helper()
	var got graphqlRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := json.Unmarshal(body, &got); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, response)
	}))
	t.Cleanup(srv.Close)

	client, err := mondoov1.NewClient(option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	require.NoError(t, err)
	return &ExtendedGqlClient{Client: client}, &got
}

// The query and mutations must match the API's GraphQL schema, which
// mondoo-go does not carry for outputs, so the documents are pinned here.

func TestGetSecurityModelRequest(t *testing.T) {
	c, got := newGraphqlTestClient(t, `{"data":{"securityModel":{
		"scopeMrn":"//captain.api.mondoo.app/organizations/my-org-123456",
		"slas":{"findings":[{"rating":"CRITICAL","daysToResolve":7,"daysBeforeWarning":5}],
			"startDateConfig":"CVE_PUBLISHED","ratingSource":"CVSS",
			"rules":[{"name":"Production","isCustom":true,
				"matchStrategy":{"tags":[{"key":"env","value":"production"}]},
				"deadlines":[{"rating":"CRITICAL","daysToResolve":3,"daysBeforeWarning":1}]}]},
		"slasSource":"ORGANIZATION"}}}`)

	model, err := c.GetSecurityModel(context.Background(), "//captain.api.mondoo.app/organizations/my-org-123456")
	require.NoError(t, err)

	assert.Equal(t,
		`query($scopeMrn:String!){securityModel(scopeMrn: $scopeMrn){scopeMrn,slas{findings{rating,daysToResolve,daysBeforeWarning},startDateConfig,ratingSource,rules{name,isCustom,matchStrategy{... on SLATagMatchStrategy{tags{key,value}}},deadlines{rating,daysToResolve,daysBeforeWarning}}},slasSource}}`,
		got.Query)
	assert.JSONEq(t, `"//captain.api.mondoo.app/organizations/my-org-123456"`, string(got.Variables["scopeMrn"]))

	require.NotNil(t, model)
	assert.Equal(t, "ORGANIZATION", model.SlasSource)
	assert.Equal(t, "CVE_PUBLISHED", model.Slas.StartDateConfig)
	assert.Equal(t, "CVSS", model.Slas.RatingSource)
	assert.Equal(t, []FindingsSLAPayload{{Rating: "CRITICAL", DaysToResolve: 7, DaysBeforeWarning: 5}}, model.Slas.Findings)

	want := SLARulePayload{
		Name:      "Production",
		IsCustom:  true,
		Deadlines: []FindingsSLAPayload{{Rating: "CRITICAL", DaysToResolve: 3, DaysBeforeWarning: 1}},
	}
	want.MatchStrategy.TagMatch.Tags = []SLATagPayload{{Key: "env", Value: "production"}}
	assert.Equal(t, []SLARulePayload{want}, model.Slas.Rules)
}

func TestGetSecurityModelNull(t *testing.T) {
	c, _ := newGraphqlTestClient(t, `{"data":{"securityModel":null}}`)

	model, err := c.GetSecurityModel(context.Background(), "//captain.api.mondoo.app/organizations/my-org-123456")
	require.NoError(t, err)
	assert.Nil(t, model)
}

func TestUpdateSecurityModelRequest(t *testing.T) {
	c, got := newGraphqlTestClient(t, `{"data":{"updateSecurityModel":{"scopeMrn":"//captain.api.mondoo.app/organizations/my-org-123456"}}}`)

	data := OrganizationSlaResourceModel{
		Critical:        organizationSlaTestObject(t, 7, 5),
		High:            organizationSlaTestObject(t, 14, 10),
		Medium:          organizationSlaTestObject(t, 45, 40),
		Low:             organizationSlaTestObject(t, 90, 0),
		StartDateConfig: types.StringValue("CVE_DETECTED"),
		RatingSource:    types.StringValue("RISK"),
		Rules: organizationSlaTestRules(t,
			organizationSlaTestRule(t, "Production", [2]string{"env", "production"}),
		),
	}
	slas, diags := expandOrganizationSLAs(context.Background(), &data)
	require.False(t, diags.HasError(), "diagnostics: %v", diags)

	err := c.UpdateSecurityModel(context.Background(), mondoov1.UpdateSecurityModelInput{
		ScopeMrn: mondoov1.String("//captain.api.mondoo.app/organizations/my-org-123456"),
		Slas:     &slas,
	})
	require.NoError(t, err)

	assert.Equal(t,
		`mutation($input:UpdateSecurityModelInput!){updateSecurityModel(input: $input){scopeMrn}}`,
		got.Query)
	assert.JSONEq(t, `{
		"scopeMrn": "//captain.api.mondoo.app/organizations/my-org-123456",
		"slas": {
			"findings": [
				{"rating": "CRITICAL", "daysToResolve": 7, "daysBeforeWarning": 5},
				{"rating": "HIGH", "daysToResolve": 14, "daysBeforeWarning": 10},
				{"rating": "MEDIUM", "daysToResolve": 45, "daysBeforeWarning": 40},
				{"rating": "LOW", "daysToResolve": 90, "daysBeforeWarning": 0}
			],
			"startDateConfig": "CVE_DETECTED",
			"ratingSource": "RISK",
			"rules": [{
				"name": "Production",
				"matchStrategy": {"tags": [{"key": "env", "value": "production"}]},
				"deadlines": [
					{"rating": "CRITICAL", "daysToResolve": 1, "daysBeforeWarning": 0},
					{"rating": "HIGH", "daysToResolve": 5, "daysBeforeWarning": 2},
					{"rating": "MEDIUM", "daysToResolve": 30, "daysBeforeWarning": 7},
					{"rating": "LOW", "daysToResolve": 60, "daysBeforeWarning": 14}
				]
			}]
		}
	}`, string(got.Variables["input"]))
}

func TestClearSecurityModelRequest(t *testing.T) {
	c, got := newGraphqlTestClient(t, `{"data":{"clearSecurityModel":{"scopeMrn":"//captain.api.mondoo.app/organizations/my-org-123456"}}}`)

	err := c.ClearSecurityModel(context.Background(), "//captain.api.mondoo.app/organizations/my-org-123456")
	require.NoError(t, err)

	assert.Equal(t,
		`mutation($input:ClearSecurityModelInput!){clearSecurityModel(input: $input){scopeMrn}}`,
		got.Query)
	assert.JSONEq(t, `{"scopeMrn": "//captain.api.mondoo.app/organizations/my-org-123456"}`, string(got.Variables["input"]))
}

func TestSecurityModelErrorIsReturned(t *testing.T) {
	c, _ := newGraphqlTestClient(t, `{"errors":[{"message":"no valid permissions to access the resource","extensions":{"code":"PermissionDenied"}}],"data":{"clearSecurityModel":null}}`)

	err := c.ClearSecurityModel(context.Background(), "//captain.api.mondoo.app/organizations/my-org-123456")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid permissions")
	assert.False(t, isNotFoundError(err))
}
