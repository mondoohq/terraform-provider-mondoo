// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mondoov1 "go.mondoo.com/mondoo-go"
)

func TestIntegrationAwsServerlessResourceGetConfigurationOptions_AccountScan(t *testing.T) {
	// An explicit setting reaches the API unchanged. An unset attribute must be
	// omitted from the request entirely so the API applies its own default of
	// true — sending false there would disable account scanning for every
	// configuration written before this attribute existed.
	for _, tc := range []struct {
		name        string
		accountScan types.Bool
		expectSent  bool
		expected    bool
	}{
		{name: "enabled", accountScan: types.BoolValue(true), expectSent: true, expected: true},
		{name: "disabled", accountScan: types.BoolValue(false), expectSent: true, expected: false},
		{name: "unset", accountScan: types.BoolNull(), expectSent: false},
		{name: "unknown", accountScan: types.BoolUnknown(), expectSent: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Every list and map must be an explicitly typed null, the way
			// Terraform decodes an unset attribute. A zero-value types.List has
			// no element type, and GetConfigurationOptions calls ElementsAs on
			// it, which dereferences that nil type and panics.
			m := integrationAwsServerlessResourceModel{
				Region:     types.StringValue("us-east-1"),
				AccountIDs: types.ListNull(types.StringType),
				ScanConfiguration: ScanConfigurationInput{
					AccountScan: tc.accountScan,
					Ec2ScanOptions: &Ec2ScanOptionsInput{
						InstanceIdsFilter:        types.ListNull(types.StringType),
						RegionsFilter:            types.ListNull(types.StringType),
						TagsFilter:               types.MapNull(types.StringType),
						ExcludeInstanceIdsFilter: types.ListNull(types.StringType),
						ExcludeRegionsFilter:     types.ListNull(types.StringType),
						ExcludeTagsFilter:        types.MapNull(types.StringType),
					},
				},
			}

			opts := m.GetConfigurationOptions()
			if !tc.expectSent {
				assert.Nil(t, opts.ScanConfiguration.AccountScan)
				return
			}
			if assert.NotNil(t, opts.ScanConfiguration.AccountScan) {
				assert.Equal(t, mondoov1.Boolean(tc.expected), *opts.ScanConfiguration.AccountScan)
			}
		})
	}
}

func TestIntegrationAwsServerlessResourceValidateConfig_Empty(t *testing.T) {
	d := &integrationAwsServerlessResourceModel{}
	diagnostics := validateIntegrationAwsServerlessResourceModel(d)
	assert.False(t, diagnostics.HasError(), "expected no errors")
}

func TestIntegrationAwsServerlessResourceValidateConfig_UseMondooVPC(t *testing.T) {
	d := &integrationAwsServerlessResourceModel{
		ScanConfiguration: ScanConfigurationInput{
			VpcConfiguration: &VPCConfigurationInput{
				UseMondooVPC: types.BoolValue(true),
			},
		},
	}

	t.Run("missing CIDR", func(t *testing.T) {
		diagnostics := validateIntegrationAwsServerlessResourceModel(d)
		if assert.True(t, diagnostics.HasError(), "expected errors") {
			assert.Equal(t, "MissingAttributeError", diagnostics[0].Summary())
		}
	})

	t.Run("with CIDR", func(t *testing.T) {
		d.ScanConfiguration.VpcConfiguration.CIDR = types.StringValue("10.0.0.0/24")
		diagnostics := validateIntegrationAwsServerlessResourceModel(d)
		assert.False(t, diagnostics.HasError(), "expected NO errors")
	})
}

func TestIntegrationAwsServerlessResourceValidateConfig_VPCFlavour(t *testing.T) {
	d := &integrationAwsServerlessResourceModel{
		ScanConfiguration: ScanConfigurationInput{
			VpcConfiguration: &VPCConfigurationInput{
				VPCFlavour: types.StringValue("DEFAULT_VPC"),
			},
		},
	}

	t.Run("default vpc flavour", func(t *testing.T) {
		diagnostics := validateIntegrationAwsServerlessResourceModel(d)
		assert.False(t, diagnostics.HasError(), "expected NO errors")
	})

	t.Run("invalid vpc flavour", func(t *testing.T) {
		d.ScanConfiguration.VpcConfiguration.VPCFlavour = types.StringValue("foo")
		diagnostics := validateIntegrationAwsServerlessResourceModel(d)
		if assert.True(t, diagnostics.HasError(), "expected errors") {
			assert.Equal(t, "InvalidAttributeValueError", diagnostics[0].Summary())
		}
	})

	t.Run("special vpc flavour that requires CIDR", func(t *testing.T) {
		d.ScanConfiguration.VpcConfiguration.VPCFlavour = types.StringValue("MONDOO_NATGW")
		diagnostics := validateIntegrationAwsServerlessResourceModel(d)
		if assert.True(t, diagnostics.HasError(), "expected errors") {
			assert.Equal(t, "MissingAttributeError", diagnostics[0].Summary())
		}

		t.Run("with CIDR", func(t *testing.T) {
			d.ScanConfiguration.VpcConfiguration.CIDR = types.StringValue("10.0.0.0/24")
			diagnostics := validateIntegrationAwsServerlessResourceModel(d)
			assert.False(t, diagnostics.HasError(), "expected NO errors")
		})
	})

	t.Run("custom vpc flavor that requires VpcTag", func(t *testing.T) {
		d.ScanConfiguration.VpcConfiguration.VPCFlavour = types.StringValue("CUSTOM_VPC")
		diagnostics := validateIntegrationAwsServerlessResourceModel(d)
		if assert.True(t, diagnostics.HasError(), "expected errors") {
			assert.Equal(t, "MissingAttributeError", diagnostics[0].Summary())
		}

		t.Run("with VpcTag", func(t *testing.T) {
			d.ScanConfiguration.VpcConfiguration.VPCTag = &VPCTagInput{
				Key:   types.StringValue("Mondoo"),
				Value: types.StringValue("true"),
			}
			diagnostics := validateIntegrationAwsServerlessResourceModel(d)
			assert.False(t, diagnostics.HasError(), "expected NO errors")
		})
	})
}

// awsServerlessModel is a model with every list and map a typed null, the way
// Terraform decodes unset attributes.
func awsServerlessModel() integrationAwsServerlessResourceModel {
	return integrationAwsServerlessResourceModel{
		Region:     types.StringValue("us-east-1"),
		AccountIDs: types.ListNull(types.StringType),
		ScanConfiguration: ScanConfigurationInput{
			Ec2ScanOptions: &Ec2ScanOptionsInput{
				InstanceIdsFilter:        types.ListNull(types.StringType),
				RegionsFilter:            types.ListNull(types.StringType),
				TagsFilter:               types.MapNull(types.StringType),
				ExcludeInstanceIdsFilter: types.ListNull(types.StringType),
				ExcludeRegionsFilter:     types.ListNull(types.StringType),
				ExcludeTagsFilter:        types.MapNull(types.StringType),
			},
		},
	}
}

func TestCrossAccountScanOptionsInput(t *testing.T) {
	t.Run("set", func(t *testing.T) {
		o := &CrossAccountScanOptionsInput{
			RoleName:      types.StringValue("MondooCrossAccountScan"),
			AccountIDs:    ConvertListValue([]string{"111111111111", "222222222222"}),
			AccountTags:   []AccountTagInput{{Key: types.StringValue("*mondoo-*"), Value: types.StringValue("scan*")}, {Key: types.StringValue("env"), Value: types.StringValue("*")}},
			EbsVolumeScan: types.BoolValue(true),
		}
		assert.Equal(t, &mondoov1.AWSCrossAccountScanOptionsInput{
			RoleName:      mondoov1.NewStringPtr("MondooCrossAccountScan"),
			AccountIDs:    &[]mondoov1.String{"111111111111", "222222222222"},
			AccountTags:   &[]mondoov1.TagInput{{Key: "*mondoo-*", Value: "scan*"}, {Key: "env", Value: "*"}},
			EbsVolumeScan: mondoov1.NewBooleanPtr(true),
		}, o.input())
	})
	t.Run("unset attributes are omitted", func(t *testing.T) {
		o := &CrossAccountScanOptionsInput{
			RoleName:      types.StringNull(),
			AccountIDs:    types.ListNull(types.StringType),
			EbsVolumeScan: types.BoolNull(),
		}
		assert.Equal(t, &mondoov1.AWSCrossAccountScanOptionsInput{}, o.input())
	})
	t.Run("an empty tag list is sent: every account is scanned", func(t *testing.T) {
		o := &CrossAccountScanOptionsInput{AccountIDs: types.ListNull(types.StringType), AccountTags: []AccountTagInput{}}
		assert.Equal(t, &[]mondoov1.TagInput{}, o.input().AccountTags)
	})
}

func TestIntegrationAwsServerlessResourceGetConfigurationOptions_CrossAccount(t *testing.T) {
	t.Run("an organization-scoped cross-account integration with WIF", func(t *testing.T) {
		m := awsServerlessModel()
		m.UseWif = types.BoolValue(true)
		m.AwsAccountID = types.StringValue("333333333333")
		m.ScanConfiguration.CrossAccountScan = types.BoolValue(true)
		m.ScanConfiguration.PropagateAccountTags = types.BoolValue(true)
		m.ScanConfiguration.CrossAccountScanOptions = &CrossAccountScanOptionsInput{
			RoleName:      types.StringValue("MondooCrossAccountScan"),
			AccountIDs:    ConvertListValue([]string{"111111111111"}),
			EbsVolumeScan: types.BoolValue(true),
		}

		opts := m.GetConfigurationOptions()
		assert.Equal(t, mondoov1.NewBooleanPtr(true), opts.UseWif)
		assert.Equal(t, mondoov1.NewStringPtr("333333333333"), opts.AwsAccountId)
		assert.Equal(t, mondoov1.NewBooleanPtr(true), opts.ScanConfiguration.CrossAccountScan)
		assert.Equal(t, mondoov1.NewBooleanPtr(true), opts.ScanConfiguration.PropagateAccountTags)
		assert.Equal(t, &mondoov1.AWSCrossAccountScanOptionsInput{
			RoleName:      mondoov1.NewStringPtr("MondooCrossAccountScan"),
			AccountIDs:    &[]mondoov1.String{"111111111111"},
			EbsVolumeScan: mondoov1.NewBooleanPtr(true),
		}, opts.ScanConfiguration.CrossAccountScanOptions)
	})
	t.Run("unset, none of it is sent", func(t *testing.T) {
		m := awsServerlessModel()
		m.UseWif = types.BoolNull()
		m.AwsAccountID = types.StringNull()
		m.ScanConfiguration.CrossAccountScan = types.BoolNull()
		m.ScanConfiguration.PropagateAccountTags = types.BoolNull()

		opts := m.GetConfigurationOptions()
		assert.Nil(t, opts.UseWif)
		assert.Nil(t, opts.AwsAccountId)
		assert.Nil(t, opts.ScanConfiguration.CrossAccountScan)
		assert.Nil(t, opts.ScanConfiguration.CrossAccountScanOptions)
		assert.Nil(t, opts.ScanConfiguration.PropagateAccountTags)
	})
}

func TestIntegrationAwsServerlessResourceValidateConfig_Wif(t *testing.T) {
	const org = "//captain.api.mondoo.app/organizations/org-1"
	const space = "//captain.api.mondoo.app/spaces/space-1"
	for _, tc := range []struct {
		name         string
		useWif       types.Bool
		awsAccountID types.String
		scopeMrn     types.String
		wantError    string
	}{
		{name: "WIF with the account", useWif: types.BoolValue(true), awsAccountID: types.StringValue("333333333333"), scopeMrn: types.StringValue(org)},
		{name: "WIF without the account", useWif: types.BoolValue(true), awsAccountID: types.StringNull(), scopeMrn: types.StringNull(), wantError: "MissingAttributeError"},
		{name: "the account without WIF", useWif: types.BoolNull(), awsAccountID: types.StringValue("333333333333"), scopeMrn: types.StringNull(), wantError: "ConflictingAttributesError"},
		{name: "organization scope without WIF", useWif: types.BoolValue(false), awsAccountID: types.StringNull(), scopeMrn: types.StringValue(org), wantError: "MissingAttributeError"},
		{name: "space scope without WIF", useWif: types.BoolNull(), awsAccountID: types.StringNull(), scopeMrn: types.StringValue(space)},
		{name: "WIF not known yet", useWif: types.BoolUnknown(), awsAccountID: types.StringNull(), scopeMrn: types.StringValue(org)},
		{name: "the account not known yet", useWif: types.BoolValue(true), awsAccountID: types.StringUnknown(), scopeMrn: types.StringValue(org)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &integrationAwsServerlessResourceModel{UseWif: tc.useWif, AwsAccountID: tc.awsAccountID, ScopeMrn: tc.scopeMrn}
			diagnostics := validateIntegrationAwsServerlessResourceModel(d)
			if tc.wantError == "" {
				assert.False(t, diagnostics.HasError(), "expected no errors: %v", diagnostics)
				return
			}
			if assert.Len(t, diagnostics, 1) {
				assert.Equal(t, tc.wantError, diagnostics[0].Summary())
			}
		})
	}
}

func TestIntegrationAwsServerlessResourceValidateConfig_ScopeAndSpace(t *testing.T) {
	const org = "//captain.api.mondoo.app/organizations/org-1"
	const space = "//captain.api.mondoo.app/spaces/space-1"
	for _, tc := range []struct {
		name     string
		scopeMrn types.String
		spaceID  types.String
		warning  bool
	}{
		{name: "space_id only", scopeMrn: types.StringNull(), spaceID: types.StringValue("space-1")},
		{name: "scope_mrn only", scopeMrn: types.StringValue(org), spaceID: types.StringNull()},
		{name: "both naming the same space", scopeMrn: types.StringValue(space), spaceID: types.StringValue("space-1")},
		{name: "both naming the same space, space_id as an MRN", scopeMrn: types.StringValue(space), spaceID: types.StringValue(space)},
		{name: "an organization next to space_id", scopeMrn: types.StringValue(org), spaceID: types.StringValue("space-1"), warning: true},
		{name: "another space next to space_id", scopeMrn: types.StringValue("//captain.api.mondoo.app/spaces/space-2"), spaceID: types.StringValue("space-1"), warning: true},
		{name: "scope_mrn not known yet", scopeMrn: types.StringUnknown(), spaceID: types.StringValue("space-1")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// use_wif keeps the WIF checks quiet for the organization scope.
			d := &integrationAwsServerlessResourceModel{ScopeMrn: tc.scopeMrn, SpaceID: tc.spaceID, UseWif: types.BoolValue(true), AwsAccountID: types.StringValue("111111111111")}
			diagnostics := validateIntegrationAwsServerlessResourceModel(d)
			assert.False(t, diagnostics.HasError(), "%v", diagnostics)
			if !tc.warning {
				assert.Empty(t, diagnostics)
				return
			}
			if assert.Len(t, diagnostics, 1) {
				assert.Equal(t, "Attribute space_id is ignored", diagnostics[0].Summary())
			}
		})
	}
}

// v048State is an integration as v0.48.0 stores it, in that release's schema,
// before scope_mrn, use_wif, aws_account_id and the cross-account attributes
// existed. Its MRN has the shape the API gives AWS integrations.
const v048State = `{
  "space_id": "space-1",
  "name": "upgrade",
  "region": "us-east-1",
  "mrn": "//integration.api.mondoo.app/spaces/space-1/aws/3KzdSxbWemWpKABpVJE2PJZ5xpM",
  "token": "token",
  "source_bucket": "mondoo-us-east-1",
  "cloud_formation_template_url": "https://example.invalid/template.json",
  "account_ids": null,
  "console_sign_in_trigger": null,
  "instance_state_change_trigger": null,
  "is_organization": null,
  "scan_configuration": {
    "account_scan": null,
    "cron_scan_in_hours": null,
    "ec2_scan": true,
    "ecr_scan": null,
    "ecs_scan": null,
    "event_scan_triggers": null,
    "vpc_configuration": null,
    "ec2_scan_options": {
      "ssm": true,
      "ebs_scan_options": null,
      "ebs_volume_scan": null,
      "exclude_instance_ids_filter": null,
      "exclude_regions_filter": null,
      "exclude_tags_filter": null,
      "instance_connect": null,
      "instance_ids_filter": null,
      "regions_filter": null,
      "tags_filter": null
    }
  }
}`

// An integration stored by v0.48.0, upgraded and planned by this build against
// the configuration that made it, plans no change; against a configuration that
// first names one of the new attributes, it is updated in place unless that
// changes what the integration is. Entirely in process: the provider's own
// protocol server, no Terraform and no Mondoo API (planning this resource does
// not call it).
func TestIntegrationAwsServerlessUpgradeFromV048(t *testing.T) {
	const typeName = "mondoo_integration_aws_serverless"
	ctx := context.Background()
	server := providerserver.NewProtocol6(New("test")())()
	schemaResp, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	require.NoError(t, err)
	resourceSchema := schemaResp.ResourceSchemas[typeName]
	require.NotNil(t, resourceSchema)
	typ := resourceSchema.ValueType()

	upgraded, err := server.UpgradeResourceState(ctx, &tfprotov6.UpgradeResourceStateRequest{
		TypeName: typeName, Version: 0, RawState: &tfprotov6.RawState{JSON: []byte(v048State)},
	})
	require.NoError(t, err)
	require.Empty(t, upgraded.Diagnostics)
	prior, err := upgraded.UpgradedState.Unmarshal(typ)
	require.NoError(t, err)

	for _, tc := range []struct {
		name      string
		extra     map[string]any // top-level attributes added to the configuration
		scan      map[string]any // scan_configuration attributes added to it
		unchanged bool
		replace   []string
	}{
		{name: "the configuration that made it", unchanged: true},
		{name: "use_wif = false, which it already is", extra: map[string]any{"use_wif": false}},
		{name: "scope_mrn naming the space it lives in, next to space_id", extra: map[string]any{"scope_mrn": "//captain.api.mondoo.app/spaces/space-1"}},
		{name: "cross-account scanning turned on", scan: map[string]any{
			"cross_account_scan":         true,
			"propagate_account_tags":     true,
			"cross_account_scan_options": map[string]any{"role_name": "MondooCrossAccountScan", "ebs_volume_scan": true},
		}},
		{name: "scope_mrn naming another space", extra: map[string]any{"scope_mrn": "//captain.api.mondoo.app/spaces/space-2"}, replace: []string{"scope_mrn"}},
		{name: "use_wif = true", extra: map[string]any{"use_wif": true, "aws_account_id": "111111111111"}, replace: []string{"use_wif", "aws_account_id"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scan := map[string]any{"ec2_scan": true, "ec2_scan_options": map[string]any{"ssm": true}}
			for k, v := range tc.scan {
				scan[k] = v
			}
			attrs := map[string]any{"space_id": "space-1", "name": "upgrade", "region": "us-east-1", "scan_configuration": scan}
			for k, v := range tc.extra {
				attrs[k] = v
			}
			configJSON, err := json.Marshal(attrs)
			require.NoError(t, err)
			config, err := (&tfprotov6.RawState{JSON: configJSON}).Unmarshal(typ)
			require.NoError(t, err)

			plan, err := server.PlanResourceChange(ctx, &tfprotov6.PlanResourceChangeRequest{
				TypeName:         typeName,
				PriorState:       upgraded.UpgradedState,
				ProposedNewState: dynamicValue(t, typ, proposedNewState(t, resourceSchema, prior, config)),
				Config:           dynamicValue(t, typ, config),
			})
			require.NoError(t, err)
			require.Empty(t, plan.Diagnostics)
			planned, err := plan.PlannedState.Unmarshal(typ)
			require.NoError(t, err)
			assert.Equal(t, tc.unchanged, planned.Equal(prior), "planned %s", planned)
			replaced := []string{}
			for _, p := range plan.RequiresReplace {
				replaced = append(replaced, string(p.Steps()[0].(tftypes.AttributeName)))
			}
			assert.ElementsMatch(t, tc.replace, replaced, "attributes that force a new integration")
		})
	}
}

// proposedNewState is what Terraform proposes before the provider plans: the
// configuration, with the prior value of every computed attribute it leaves
// unset. This resource's computed attributes are all top-level, so that is the
// whole of Terraform's rule here.
func proposedNewState(t *testing.T, s *tfprotov6.Schema, prior, config tftypes.Value) tftypes.Value {
	var priorAttrs, configAttrs map[string]tftypes.Value
	require.NoError(t, prior.As(&priorAttrs))
	require.NoError(t, config.As(&configAttrs))
	for _, a := range s.Block.Attributes {
		if a.Computed && configAttrs[a.Name].IsNull() {
			configAttrs[a.Name] = priorAttrs[a.Name]
		}
	}
	return tftypes.NewValue(config.Type(), configAttrs)
}

func dynamicValue(t *testing.T, typ tftypes.Type, v tftypes.Value) *tfprotov6.DynamicValue {
	dv, err := tfprotov6.NewDynamicValue(typ, v)
	require.NoError(t, err)
	return &dv
}

func TestAwsIntegrationScopeMrn(t *testing.T) {
	for _, tc := range []struct{ mrn, scope string }{
		{mrn: "//integration.api.mondoo.app/spaces/space-1/aws/3KzdSxbWemWpKABpVJE2PJZ5xpM", scope: "//captain.api.mondoo.app/spaces/space-1"},
		{mrn: "//integration.api.mondoo.app/organizations/org-1/aws/3KzdSxbWemWpKABpVJE2PJZ5xpM", scope: "//captain.api.mondoo.app/organizations/org-1"},
		{mrn: "//integration.api.mondoo.app/spaces/space-1/integrations/int-1", scope: "//captain.api.mondoo.app/spaces/space-1"},
		{mrn: "//integration.api.mondoo.app/aws/3KzdSxbWemWpKABpVJE2PJZ5xpM", scope: ""},
		{mrn: "not-an-mrn", scope: ""},
		{mrn: "", scope: ""},
	} {
		assert.Equal(t, tc.scope, awsIntegrationScopeMrn(tc.mrn), tc.mrn)
	}
}

// replaceCheckState is the prior state the replace checks see: an existing
// integration, of which they only read the MRN.
func replaceCheckState(integrationMrn string) (tfsdk.State, tfsdk.Plan) {
	s := schema.Schema{Attributes: map[string]schema.Attribute{"mrn": schema.StringAttribute{Computed: true}}}
	raw := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{"mrn": tftypes.String}},
		map[string]tftypes.Value{"mrn": tftypes.NewValue(tftypes.String, integrationMrn)})
	return tfsdk.State{Schema: s, Raw: raw}, tfsdk.Plan{Schema: s, Raw: raw}
}

func TestIntegrationAwsServerlessScopeMrnRequiresReplace(t *testing.T) {
	const integrationMrn = "//integration.api.mondoo.app/spaces/space-1/aws/int-1"
	space := types.StringValue("//captain.api.mondoo.app/spaces/space-1")
	otherSpace := types.StringValue("//captain.api.mondoo.app/spaces/space-2")
	for _, tc := range []struct {
		name        string
		state, plan types.String
		replace     bool
	}{
		{name: "a state from before scope_mrn, not naming it", state: types.StringNull(), plan: types.StringNull()},
		{name: "a state from before scope_mrn, naming the space its MRN names", state: types.StringNull(), plan: space},
		{name: "a state from before scope_mrn, naming another space", state: types.StringNull(), plan: otherSpace, replace: true},
		{name: "the same scope", state: space, plan: space},
		{name: "another scope", state: space, plan: otherSpace, replace: true},
		{name: "a scope not known yet", state: space, plan: types.StringUnknown(), replace: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, plan := replaceCheckState(integrationMrn)
			req := planmodifier.StringRequest{State: state, Plan: plan, StateValue: tc.state, PlanValue: tc.plan, ConfigValue: tc.plan}
			resp := &planmodifier.StringResponse{PlanValue: tc.plan}
			scopeMrnRequiresReplace.PlanModifyString(context.Background(), req, resp)
			assert.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
			assert.Equal(t, tc.replace, resp.RequiresReplace)
		})
	}
}

func TestIntegrationAwsServerlessUseWifRequiresReplace(t *testing.T) {
	for _, tc := range []struct {
		name        string
		state, plan types.Bool
		replace     bool
	}{
		{name: "unset stays unset", state: types.BoolNull(), plan: types.BoolNull()},
		{name: "unset becomes false", state: types.BoolNull(), plan: types.BoolValue(false)},
		{name: "false becomes unset", state: types.BoolValue(false), plan: types.BoolNull()},
		{name: "unset becomes true", state: types.BoolNull(), plan: types.BoolValue(true), replace: true},
		{name: "true becomes false", state: types.BoolValue(true), plan: types.BoolValue(false), replace: true},
		{name: "true becomes unset", state: types.BoolValue(true), plan: types.BoolNull(), replace: true},
		{name: "not known yet", state: types.BoolNull(), plan: types.BoolUnknown(), replace: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, plan := replaceCheckState("//integration.api.mondoo.app/spaces/space-1/aws/int-1")
			req := planmodifier.BoolRequest{State: state, Plan: plan, StateValue: tc.state, PlanValue: tc.plan, ConfigValue: tc.plan}
			resp := &planmodifier.BoolResponse{PlanValue: tc.plan}
			useWifRequiresReplace.PlanModifyBool(context.Background(), req, resp)
			assert.Equal(t, tc.replace, resp.RequiresReplace)
		})
	}
}

func TestIntegrationAwsServerlessAwsAccountIDRequiresReplace(t *testing.T) {
	for _, tc := range []struct {
		name        string
		state, plan types.String
		replace     bool
	}{
		{name: "unset stays unset", state: types.StringNull(), plan: types.StringNull()},
		{name: "unset becomes empty", state: types.StringNull(), plan: types.StringValue("")},
		{name: "the same account", state: types.StringValue("111111111111"), plan: types.StringValue("111111111111")},
		{name: "unset becomes an account", state: types.StringNull(), plan: types.StringValue("111111111111"), replace: true},
		{name: "another account", state: types.StringValue("111111111111"), plan: types.StringValue("222222222222"), replace: true},
		{name: "not known yet", state: types.StringValue("111111111111"), plan: types.StringUnknown(), replace: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			state, plan := replaceCheckState("//integration.api.mondoo.app/spaces/space-1/aws/int-1")
			req := planmodifier.StringRequest{State: state, Plan: plan, StateValue: tc.state, PlanValue: tc.plan, ConfigValue: tc.plan}
			resp := &planmodifier.StringResponse{PlanValue: tc.plan}
			awsAccountIDRequiresReplace.PlanModifyString(context.Background(), req, resp)
			assert.Equal(t, tc.replace, resp.RequiresReplace)
		})
	}
}
