// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
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

func TestIntegrationAwsServerlessResourceGetConfigurationOptions_ScanOptions(t *testing.T) {
	model := func(sc ScanConfigurationInput) integrationAwsServerlessResourceModel {
		sc.Ec2ScanOptions = &Ec2ScanOptionsInput{
			InstanceIdsFilter:        types.ListNull(types.StringType),
			RegionsFilter:            types.ListNull(types.StringType),
			TagsFilter:               types.MapNull(types.StringType),
			ExcludeInstanceIdsFilter: types.ListNull(types.StringType),
			ExcludeRegionsFilter:     types.ListNull(types.StringType),
			ExcludeTagsFilter:        types.MapNull(types.StringType),
		}
		return integrationAwsServerlessResourceModel{
			Region:            types.StringValue("us-east-1"),
			AccountIDs:        types.ListNull(types.StringType),
			ScanConfiguration: sc,
		}
	}

	t.Run("unset options are omitted", func(t *testing.T) {
		sc := model(ScanConfigurationInput{
			DiscoveryTargets: types.ListNull(types.StringType),
		}).GetConfigurationOptions().ScanConfiguration

		assert.Nil(t, sc.CronScanExpression)
		assert.Nil(t, sc.CrossAccountScan)
		assert.Nil(t, sc.CrossAccountScanOptions)
		assert.Nil(t, sc.PropagateAccountTags)
		assert.Nil(t, sc.DiscoveryTargets)
		assert.Nil(t, sc.Scheduling)
	})

	t.Run("set options are sent", func(t *testing.T) {
		sc := model(ScanConfigurationInput{
			CronScanExpression: types.StringValue("0 3 * * MON-FRI"),
			CrossAccountScan:   types.BoolValue(true),
			CrossAccountScanOptions: &CrossAccountScanOptionsInput{
				RoleName: types.StringValue("MondooCrossAccountRole"),
				AccountIDs: types.ListValueMust(types.StringType, []attr.Value{
					types.StringValue("111111111111"),
				}),
				AccountTags: types.MapValueMust(types.StringType, map[string]attr.Value{
					"team": types.StringValue("*"),
					"env":  types.StringValue("prod"),
				}),
				Organizations: types.MapValueMust(types.StringType, map[string]attr.Value{
					"o-abc123defg": types.StringValue("222222222222"),
				}),
				EbsVolumeScan: types.BoolValue(true),
			},
			PropagateAccountTags: types.BoolValue(false),
			DiscoveryTargets: types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue("accounts"),
				types.StringValue("s3-buckets"),
			}),
			Scheduling: types.StringValue("SERVER"),
		}).GetConfigurationOptions().ScanConfiguration

		assert.Equal(t, mondoov1.NewStringPtr("0 3 * * MON-FRI"), sc.CronScanExpression)
		assert.Equal(t, mondoov1.NewBooleanPtr(true), sc.CrossAccountScan)
		assert.Equal(t, mondoov1.NewBooleanPtr(false), sc.PropagateAccountTags)
		assert.Equal(t, &[]mondoov1.String{"accounts", "s3-buckets"}, sc.DiscoveryTargets)
		assert.Equal(t, ToPtr(mondoov1.ScanSchedulingServer), sc.Scheduling)
		assert.Equal(t, &mondoov1.AWSCrossAccountScanOptionsInput{
			RoleName:   mondoov1.NewStringPtr("MondooCrossAccountRole"),
			AccountIDs: &[]mondoov1.String{"111111111111"},
			AccountTags: &[]mondoov1.TagInput{
				{Key: "env", Value: "prod"},
				{Key: "team", Value: "*"},
			},
			Organizations: &[]mondoov1.AWSCrossAccountOrganizationInput{
				{OrganizationId: "o-abc123defg", ManagementAccountId: "222222222222"},
			},
			EbsVolumeScan: mondoov1.NewBooleanPtr(true),
		}, sc.CrossAccountScanOptions)
	})
}

func TestIntegrationAwsServerlessResourceSchema_CrossAccountMapsRejectNullValues(t *testing.T) {
	// A null element makes ElementsAs fail for the whole map and leave it
	// empty, which would drop the account_tags filter and scan every account.
	// The schema must reject it at plan time.
	var resp resource.SchemaResponse
	(&integrationAwsServerlessResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "schema diagnostics: %v", resp.Diagnostics)

	options := resp.Schema.Attributes["scan_configuration"].(schema.SingleNestedAttribute).
		Attributes["cross_account_scan_options"].(schema.SingleNestedAttribute)

	validate := func(attribute schema.MapAttribute, elements map[string]attr.Value) diag.Diagnostics {
		req := validator.MapRequest{ConfigValue: types.MapValueMust(types.StringType, elements)}
		resp := &validator.MapResponse{}
		for _, v := range attribute.Validators {
			v.ValidateMap(context.Background(), req, resp)
		}
		return resp.Diagnostics
	}

	for _, name := range []string{"account_tags", "organizations"} {
		t.Run(name, func(t *testing.T) {
			attribute := options.Attributes[name].(schema.MapAttribute)

			assert.True(t, validate(attribute, map[string]attr.Value{
				"set":   types.StringValue("value"),
				"unset": types.StringNull(),
			}).HasError(), "a null element must be rejected")

			assert.False(t, validate(attribute, map[string]attr.Value{
				"set": types.StringValue("value"),
			}).HasError(), "a map without null elements must pass")
		})
	}
}

func TestIntegrationAwsServerlessResourceGetConfigurationOptions_Wif(t *testing.T) {
	model := func(useWif types.Bool, awsAccountID types.String) integrationAwsServerlessResourceModel {
		return integrationAwsServerlessResourceModel{
			Region:       types.StringValue("us-east-1"),
			AccountIDs:   types.ListNull(types.StringType),
			UseWif:       useWif,
			AwsAccountID: awsAccountID,
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

	t.Run("set, both are sent", func(t *testing.T) {
		m := model(types.BoolValue(true), types.StringValue("111111111111"))
		opts := m.GetConfigurationOptions()
		assert.Equal(t, mondoov1.NewBooleanPtr(true), opts.UseWif)
		assert.Equal(t, mondoov1.NewStringPtr("111111111111"), opts.AwsAccountId)
	})

	t.Run("unset, neither is sent", func(t *testing.T) {
		m := model(types.BoolNull(), types.StringNull())
		opts := m.GetConfigurationOptions()
		assert.Nil(t, opts.UseWif)
		assert.Nil(t, opts.AwsAccountId)
	})
}

func TestIntegrationAwsServerlessResourceValidateConfig_Wif(t *testing.T) {
	for _, tc := range []struct {
		name         string
		useWif       types.Bool
		awsAccountID types.String
		wantError    string
	}{
		{name: "WIF with the account", useWif: types.BoolValue(true), awsAccountID: types.StringValue("333333333333")},
		{name: "WIF without the account", useWif: types.BoolValue(true), awsAccountID: types.StringNull(), wantError: "MissingAttributeError"},
		{name: "the account without WIF", useWif: types.BoolNull(), awsAccountID: types.StringValue("333333333333"), wantError: "ConflictingAttributesError"},
		{name: "neither", useWif: types.BoolNull(), awsAccountID: types.StringNull()},
		{name: "WIF not known yet", useWif: types.BoolUnknown(), awsAccountID: types.StringNull()},
		{name: "the account not known yet", useWif: types.BoolValue(true), awsAccountID: types.StringUnknown()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &integrationAwsServerlessResourceModel{UseWif: tc.useWif, AwsAccountID: tc.awsAccountID}
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

// v048State is an integration as v0.48.0 stores it, in that release's schema,
// before use_wif, aws_account_id and wif_config existed. Its MRN has the shape
// the API gives AWS integrations.
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
// the configuration that made it, plans no change; use_wif = false updates it
// in place, use_wif = true makes a new integration. Entirely in process: the
// provider's own protocol server, no Terraform and no Mondoo API (planning this
// resource does not call it).
func TestIntegrationAwsServerlessUpgradeFromV048(t *testing.T) {
	p := newAwsServerlessProtocol(t)
	upgraded, err := p.server.UpgradeResourceState(context.Background(), &tfprotov6.UpgradeResourceStateRequest{
		TypeName: awsServerlessTypeName, Version: 0, RawState: &tfprotov6.RawState{JSON: []byte(v048State)},
	})
	require.NoError(t, err)
	require.Empty(t, upgraded.Diagnostics)
	prior, err := upgraded.UpgradedState.Unmarshal(p.typ)
	require.NoError(t, err)

	for _, tc := range []struct {
		name      string
		extra     map[string]any // top-level attributes added to the configuration
		unchanged bool
		replace   []string
	}{
		{name: "the configuration that made it", unchanged: true},
		{name: "use_wif = false, which it already is", extra: map[string]any{"use_wif": false}},
		{name: "use_wif = true", extra: map[string]any{"use_wif": true, "aws_account_id": "111111111111"}, replace: []string{"use_wif", "aws_account_id"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attrs := map[string]any{
				"space_id":           "space-1",
				"name":               "upgrade",
				"region":             "us-east-1",
				"scan_configuration": map[string]any{"ec2_scan": true, "ec2_scan_options": map[string]any{"ssm": true}},
			}
			for k, v := range tc.extra {
				attrs[k] = v
			}
			planned, replaced := p.plan(t, prior, attrs)
			assert.Equal(t, tc.unchanged, planned.Equal(prior), "planned %s", planned)
			assert.ElementsMatch(t, tc.replace, replaced, "attributes that force a new integration")
		})
	}
}

// A WIF integration, imported and then planned against the configuration that
// describes it, is updated in place: the import takes over use_wif and
// aws_account_id, which cannot change in place.
func TestIntegrationAwsServerlessImportedWifPlansNoReplace(t *testing.T) {
	p := newAwsServerlessProtocol(t)
	imported := importedAwsServerlessModel(Integration{
		Mrn:  "//integration.api.mondoo.app/spaces/space-1/aws/3KzdSxbWemWpKABpVJE2PJZ5xpM",
		Name: "imported",
		ConfigurationOptions: ClientIntegrationConfigurationOptions{
			AWSConfigurationOptions: AWSConfigurationOptions{UseWif: true, AwsAccountId: "111111111111"},
		},
	})
	_, replaced := p.plan(t, p.stateOf(t, imported), map[string]any{
		"space_id":           "space-1",
		"name":               "imported",
		"region":             "us-east-1",
		"use_wif":            true,
		"aws_account_id":     "111111111111",
		"scan_configuration": map[string]any{"ec2_scan_options": map[string]any{}},
	})
	assert.Empty(t, replaced, "attributes that force a new integration")
}

func TestImportedAwsServerlessModel(t *testing.T) {
	const integrationMrn = "//integration.api.mondoo.app/spaces/space-1/aws/int-1"
	wif := ClientIntegrationConfigurationOptions{AWSConfigurationOptions: AWSConfigurationOptions{UseWif: true, AwsAccountId: "111111111111"}}
	for _, tc := range []struct {
		name        string
		integration Integration
		want        integrationAwsServerlessResourceModel
	}{
		{
			name:        "an integration without WIF",
			integration: Integration{Mrn: integrationMrn, Name: "one"},
			want: integrationAwsServerlessResourceModel{
				Mrn: types.StringValue(integrationMrn), Name: types.StringValue("one"), SpaceID: types.StringValue("space-1"),
			},
		},
		{
			name:        "an integration with WIF",
			integration: Integration{Mrn: integrationMrn, Name: "one", ConfigurationOptions: wif},
			want: integrationAwsServerlessResourceModel{
				Mrn: types.StringValue(integrationMrn), Name: types.StringValue("one"), SpaceID: types.StringValue("space-1"),
				UseWif: types.BoolValue(true), AwsAccountID: types.StringValue("111111111111"),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Every import carries a typed null account_ids.
			tc.want.AccountIDs = types.ListNull(types.StringType)
			assert.Equal(t, tc.want, importedAwsServerlessModel(tc.integration))
		})
	}
}

func TestIntegrationAwsServerlessAwsAccountIDIsAnAccount(t *testing.T) {
	p := newAwsServerlessProtocol(t)
	for _, tc := range []struct {
		accountID string
		valid     bool
	}{
		{accountID: "111111111111", valid: true},
		{accountID: "11111111111"},
		{accountID: "1111111111111"},
		{accountID: "11111111111a"},
		{accountID: " 111111111111"},
	} {
		t.Run(tc.accountID, func(t *testing.T) {
			configJSON, err := json.Marshal(map[string]any{
				"name":               "one",
				"region":             "us-east-1",
				"use_wif":            true,
				"aws_account_id":     tc.accountID,
				"scan_configuration": map[string]any{"ec2_scan_options": map[string]any{}},
			})
			require.NoError(t, err)
			config, err := (&tfprotov6.RawState{JSON: configJSON}).Unmarshal(p.typ)
			require.NoError(t, err)
			resp, err := p.server.ValidateResourceConfig(context.Background(), &tfprotov6.ValidateResourceConfigRequest{
				TypeName: awsServerlessTypeName,
				Config:   dynamicValue(t, p.typ, config),
			})
			require.NoError(t, err)
			errs := []string{}
			for _, d := range resp.Diagnostics {
				if d.Severity == tfprotov6.DiagnosticSeverityError {
					errs = append(errs, d.Detail)
				}
			}
			if tc.valid {
				assert.Empty(t, errs)
				return
			}
			if assert.Len(t, errs, 1) {
				assert.Contains(t, errs[0], "must be a 12-digit AWS account ID")
			}
		})
	}
}

const awsServerlessTypeName = "mondoo_integration_aws_serverless"

// awsServerlessProtocol drives the resource through the provider's own protocol
// server, as Terraform does, with no Terraform and no Mondoo API: planning this
// resource does not call it.
type awsServerlessProtocol struct {
	server tfprotov6.ProviderServer
	schema *tfprotov6.Schema
	typ    tftypes.Type
}

func newAwsServerlessProtocol(t *testing.T) awsServerlessProtocol {
	server := providerserver.NewProtocol6(New("test")())()
	schemaResp, err := server.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	require.NoError(t, err)
	s := schemaResp.ResourceSchemas[awsServerlessTypeName]
	require.NotNil(t, s)
	return awsServerlessProtocol{server: server, schema: s, typ: s.ValueType()}
}

// plan plans the configuration attrs against prior and returns the planned
// state and the attributes that force a new integration.
func (p awsServerlessProtocol) plan(t *testing.T, prior tftypes.Value, attrs map[string]any) (tftypes.Value, []string) {
	configJSON, err := json.Marshal(attrs)
	require.NoError(t, err)
	config, err := (&tfprotov6.RawState{JSON: configJSON}).Unmarshal(p.typ)
	require.NoError(t, err)
	resp, err := p.server.PlanResourceChange(context.Background(), &tfprotov6.PlanResourceChangeRequest{
		TypeName:         awsServerlessTypeName,
		PriorState:       dynamicValue(t, p.typ, prior),
		ProposedNewState: dynamicValue(t, p.typ, proposedNewState(t, p.schema, prior, config)),
		Config:           dynamicValue(t, p.typ, config),
	})
	require.NoError(t, err)
	for _, d := range resp.Diagnostics {
		require.NotEqual(t, tfprotov6.DiagnosticSeverityError, d.Severity, "%s: %s", d.Summary, d.Detail)
	}
	planned, err := resp.PlannedState.Unmarshal(p.typ)
	require.NoError(t, err)
	replaced := []string{}
	for _, path := range resp.RequiresReplace {
		replaced = append(replaced, string(path.Steps()[0].(tftypes.AttributeName)))
	}
	return planned, replaced
}

// stateOf is the state the resource would store for model.
func (p awsServerlessProtocol) stateOf(t *testing.T, model integrationAwsServerlessResourceModel) tftypes.Value {
	var schemaResp resource.SchemaResponse
	(&integrationAwsServerlessResource{}).Schema(context.Background(), resource.SchemaRequest{}, &schemaResp)
	state := tfsdk.State{Schema: schemaResp.Schema, Raw: tftypes.NewValue(p.typ, nil)}
	require.Empty(t, state.Set(context.Background(), &model))
	return state.Raw
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

// quickcreateUrl builds a CloudFormation quickcreate link the way the server
// does: the parameters are the query of the fragment, each value query-escaped.
func quickcreateUrl(params ...string) string {
	return "https://us-east-1.console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/quickcreate?" +
		strings.Join(append([]string{"templateURL=" + url.QueryEscape("https://example.invalid/template.json"), "stackName=mondoo"}, params...), "&")
}

func TestWifConfigFromQuickcreateUrl(t *testing.T) {
	// Base64 with the characters query escaping changes: + / =
	const wifConfig = "eyJ0eXBlIjoiZXh0ZXJuYWxfYWNjb3VudCJ9+abc/def=="
	for _, tc := range []struct{ name, link, want string }{
		{name: "a WIF integration's link", link: quickcreateUrl("param_MondooWifConfig="+url.QueryEscape(wifConfig), "param_MondooIntegrationMrn=mrn"), want: wifConfig},
		{name: "a token integration's link", link: quickcreateUrl("param_MondooIntegrationMrn=mrn", "param_MondooToken=")},
		{name: "a link without parameters", link: "https://us-east-1.console.aws.amazon.com/cloudformation/home?region=us-east-1#/stacks/quickcreate"},
		{name: "not a link", link: "%zz"},
		{name: "empty", link: ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, wifConfigFromQuickcreateUrl(tc.link))
		})
	}
}

func TestIntegrationAwsServerlessSetComputedFrom_WifConfig(t *testing.T) {
	withWif := quickcreateUrl("param_MondooWifConfig=" + url.QueryEscape("config"))
	withoutWif := quickcreateUrl("param_MondooToken=")
	for _, tc := range []struct {
		name   string
		before types.String
		link   string
		want   types.String
	}{
		{name: "created with WIF", before: types.StringUnknown(), link: withWif, want: types.StringValue("config")},
		{name: "created without WIF", before: types.StringUnknown(), link: withoutWif, want: types.StringNull()},
		{name: "a refresh fills it in", before: types.StringNull(), link: withWif, want: types.StringValue("config")},
		{name: "a link that lost it leaves it as it was", before: types.StringValue("config"), link: withoutWif, want: types.StringValue("config")},
		{name: "once known it stays", before: types.StringValue("config"), link: quickcreateUrl("param_MondooWifConfig=other"), want: types.StringValue("config")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := integrationAwsServerlessResourceModel{WifConfig: tc.before}
			m.setComputedFrom(Integration{ConfigurationOptions: ClientIntegrationConfigurationOptions{
				AWSConfigurationOptions: AWSConfigurationOptions{CloudFormationQuickcreateUrl: tc.link},
			}})
			assert.Equal(t, tc.want, m.WifConfig)
		})
	}
}

// replaceCheckState is a minimal state and plan for a RequiresReplaceIf
// modifier, which only runs for an existing resource.
func replaceCheckState(integrationMrn string) (tfsdk.State, tfsdk.Plan) {
	s := schema.Schema{Attributes: map[string]schema.Attribute{"mrn": schema.StringAttribute{Computed: true}}}
	raw := tftypes.NewValue(tftypes.Object{AttributeTypes: map[string]tftypes.Type{"mrn": tftypes.String}},
		map[string]tftypes.Value{"mrn": tftypes.NewValue(tftypes.String, integrationMrn)})
	return tfsdk.State{Schema: s, Raw: raw}, tfsdk.Plan{Schema: s, Raw: raw}
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
