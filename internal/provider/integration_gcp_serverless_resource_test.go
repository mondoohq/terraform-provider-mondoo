// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mondoov1 "go.mondoo.com/mondoo-go"
)

func boolPtr(b bool) *bool { return &b }

// assertBoolPtr asserts a *mondoov1.Boolean equals the expected *bool, treating
// nil as "field omitted from the request".
func assertBoolPtr(t *testing.T, want *bool, got *mondoov1.Boolean) {
	t.Helper()
	if want == nil {
		assert.Nil(t, got)
		return
	}
	require.NotNil(t, got)
	assert.EqualValues(t, *want, bool(*got))
}

func TestIntegrationGcpServerlessGetConfigurationOptions_Minimal(t *testing.T) {
	m := integrationGcpServerlessResourceModel{
		Scope:         types.StringValue("123456789012"),
		HostProjectID: types.StringValue("my-host-project"),
		Region:        types.StringValue("us-central1"),
	}

	opts := m.GetConfigurationOptions()
	require.NotNil(t, opts)
	require.NotNil(t, opts.Scope)
	assert.EqualValues(t, "123456789012", *opts.Scope)
	assert.EqualValues(t, "my-host-project", opts.HostProjectId)
	assert.EqualValues(t, "us-central1", opts.Region)
	// No supplied identity => the field is omitted from the request.
	assert.Nil(t, opts.SuppliedSaIdentity)
	// None of the WIF / cross-org optionals are set => all omitted.
	assert.Nil(t, opts.CrossOrg)
	assert.Nil(t, opts.UseWif)
	assert.Nil(t, opts.ServiceAccountId)
	// No scan_configuration block => no scan configuration sent.
	assert.Nil(t, opts.ScanConfiguration)
}

// TestIntegrationGcpServerlessGetConfigurationOptions_OptionalBoolFields covers
// the tri-state semantics of the optional boolean fields: unset (null) is
// omitted from the request, while an explicit true OR false is sent through.
// Sending an explicit false matters — it must not be conflated with "unset".
func TestIntegrationGcpServerlessGetConfigurationOptions_OptionalBoolFields(t *testing.T) {
	base := func() integrationGcpServerlessResourceModel {
		return integrationGcpServerlessResourceModel{
			HostProjectID: types.StringValue("my-host-project"),
			Region:        types.StringValue("us-central1"),
		}
	}

	t.Run("cross_org", func(t *testing.T) {
		cases := []struct {
			name string
			in   types.Bool
			want *bool
		}{
			{"unset is omitted", types.BoolNull(), nil},
			{"explicit true is sent", types.BoolValue(true), boolPtr(true)},
			{"explicit false is sent", types.BoolValue(false), boolPtr(false)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				m := base()
				m.CrossOrg = tc.in
				got := m.GetConfigurationOptions().CrossOrg
				assertBoolPtr(t, tc.want, got)
			})
		}
	})

	t.Run("use_wif", func(t *testing.T) {
		cases := []struct {
			name string
			in   types.Bool
			want *bool
		}{
			{"unset is omitted", types.BoolNull(), nil},
			{"explicit true is sent", types.BoolValue(true), boolPtr(true)},
			{"explicit false is sent", types.BoolValue(false), boolPtr(false)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				m := base()
				m.UseWif = tc.in
				got := m.GetConfigurationOptions().UseWif
				assertBoolPtr(t, tc.want, got)
			})
		}
	})

	t.Run("propagate_project_tags", func(t *testing.T) {
		cases := []struct {
			name string
			in   types.Bool
			want *bool
		}{
			{"unset is omitted", types.BoolNull(), nil},
			{"explicit true is sent", types.BoolValue(true), boolPtr(true)},
			{"explicit false is sent", types.BoolValue(false), boolPtr(false)},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				m := base()
				m.ScanConfiguration = &GcpServerlessScanConfigurationInput{
					TagsFilter:           types.MapNull(types.StringType),
					ExcludedTagsFilter:   types.MapNull(types.StringType),
					PropagateProjectTags: tc.in,
				}
				sc := m.GetConfigurationOptions().ScanConfiguration
				require.NotNil(t, sc)
				assertBoolPtr(t, tc.want, sc.PropagateProjectTags)
			})
		}
	})
}

// TestIntegrationGcpServerlessGetConfigurationOptions_ServiceAccountId covers
// the optional numeric service account id used as the WIF binding subject.
func TestIntegrationGcpServerlessGetConfigurationOptions_ServiceAccountId(t *testing.T) {
	t.Run("unset is omitted", func(t *testing.T) {
		m := integrationGcpServerlessResourceModel{
			HostProjectID: types.StringValue("my-host-project"),
			Region:        types.StringValue("us-central1"),
		}
		assert.Nil(t, m.GetConfigurationOptions().ServiceAccountId)
	})

	t.Run("supplied is sent", func(t *testing.T) {
		m := integrationGcpServerlessResourceModel{
			HostProjectID:    types.StringValue("my-host-project"),
			Region:           types.StringValue("us-central1"),
			ServiceAccountID: types.StringValue("123456789012345678901"),
		}
		got := m.GetConfigurationOptions().ServiceAccountId
		require.NotNil(t, got)
		assert.EqualValues(t, "123456789012345678901", *got)
	})
}

// TestIntegrationGcpServerlessGetConfigurationOptions_WifCrossOrgSupplied is a
// full "everything supplied" WIF + cross-org integration.
func TestIntegrationGcpServerlessGetConfigurationOptions_WifCrossOrgSupplied(t *testing.T) {
	m := integrationGcpServerlessResourceModel{
		Scope:            types.StringValue("organizations/123456789012"),
		HostProjectID:    types.StringValue("my-host-project"),
		Region:           types.StringValue("us-central1"),
		CrossOrg:         types.BoolValue(true),
		UseWif:           types.BoolValue(true),
		ServiceAccountID: types.StringValue("123456789012345678901"),
		ScanConfiguration: &GcpServerlessScanConfigurationInput{
			TagsFilter:           types.MapNull(types.StringType),
			ExcludedTagsFilter:   types.MapNull(types.StringType),
			PropagateProjectTags: types.BoolValue(true),
		},
	}

	opts := m.GetConfigurationOptions()
	require.NotNil(t, opts)
	require.NotNil(t, opts.CrossOrg)
	assert.EqualValues(t, true, *opts.CrossOrg)
	require.NotNil(t, opts.UseWif)
	assert.EqualValues(t, true, *opts.UseWif)
	require.NotNil(t, opts.ServiceAccountId)
	assert.EqualValues(t, "123456789012345678901", *opts.ServiceAccountId)
	require.NotNil(t, opts.ScanConfiguration)
	require.NotNil(t, opts.ScanConfiguration.PropagateProjectTags)
	assert.EqualValues(t, true, *opts.ScanConfiguration.PropagateProjectTags)
}

// TestValidateGcpServerlessConfig covers the plan-time WIF / cross-org
// precondition checks.
func TestValidateGcpServerlessConfig(t *testing.T) {
	cases := []struct {
		name    string
		model   integrationGcpServerlessResourceModel
		wantErr bool
	}{
		{
			name:  "no wif, no cross_org — valid",
			model: integrationGcpServerlessResourceModel{HostProjectID: types.StringValue("p"), Region: types.StringValue("r")},
		},
		{
			name: "use_wif with service_account_id — valid",
			model: integrationGcpServerlessResourceModel{
				UseWif:           types.BoolValue(true),
				ServiceAccountID: types.StringValue("123456789012345678901"),
			},
		},
		{
			name:    "use_wif without service_account_id — error",
			model:   integrationGcpServerlessResourceModel{UseWif: types.BoolValue(true)},
			wantErr: true,
		},
		{
			name:    "use_wif with empty service_account_id — error",
			model:   integrationGcpServerlessResourceModel{UseWif: types.BoolValue(true), ServiceAccountID: types.StringValue("")},
			wantErr: true,
		},
		{
			name: "use_wif with unknown service_account_id — skipped (valid)",
			model: integrationGcpServerlessResourceModel{
				UseWif:           types.BoolValue(true),
				ServiceAccountID: types.StringUnknown(),
			},
		},
		{
			name: "cross_org with use_wif — valid",
			model: integrationGcpServerlessResourceModel{
				CrossOrg:         types.BoolValue(true),
				UseWif:           types.BoolValue(true),
				ServiceAccountID: types.StringValue("123456789012345678901"),
			},
		},
		{
			name:    "cross_org without use_wif — error",
			model:   integrationGcpServerlessResourceModel{CrossOrg: types.BoolValue(true), UseWif: types.BoolValue(false)},
			wantErr: true,
		},
		{
			name: "cross_org with unknown use_wif — skipped (valid)",
			model: integrationGcpServerlessResourceModel{
				CrossOrg: types.BoolValue(true),
				UseWif:   types.BoolUnknown(),
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diags := validateGcpServerlessConfig(&tc.model)
			assert.Equal(t, tc.wantErr, diags.HasError(), "diagnostics: %v", diags)
		})
	}
}

func TestIntegrationGcpServerlessGetConfigurationOptions_OmittedScope(t *testing.T) {
	m := integrationGcpServerlessResourceModel{
		HostProjectID: types.StringValue("my-host-project"),
		Region:        types.StringValue("us-central1"),
	}

	opts := m.GetConfigurationOptions()
	require.NotNil(t, opts)
	// Omitted scope is not sent; the scanner falls back to its default scope.
	assert.Nil(t, opts.Scope)
}

func TestIntegrationGcpServerlessGetConfigurationOptions_SuppliedSaIdentity(t *testing.T) {
	m := integrationGcpServerlessResourceModel{
		HostProjectID:      types.StringValue("my-host-project"),
		Region:             types.StringValue("us-central1"),
		SuppliedSaIdentity: types.StringValue("byoi-sa@my-host-project.iam.gserviceaccount.com"),
	}

	opts := m.GetConfigurationOptions()
	require.NotNil(t, opts)
	require.NotNil(t, opts.SuppliedSaIdentity)
	assert.EqualValues(t, "byoi-sa@my-host-project.iam.gserviceaccount.com", *opts.SuppliedSaIdentity)
}

func TestIntegrationGcpServerlessGetConfigurationOptions_ScanScheduleHours(t *testing.T) {
	m := integrationGcpServerlessResourceModel{
		HostProjectID: types.StringValue("my-host-project"),
		Region:        types.StringValue("us-central1"),
		ScanConfiguration: &GcpServerlessScanConfigurationInput{
			TagsFilter:         types.MapNull(types.StringType),
			ExcludedTagsFilter: types.MapNull(types.StringType),
			ScanScheduleHours:  types.Int32Value(12),
		},
	}

	opts := m.GetConfigurationOptions()
	require.NotNil(t, opts)
	require.NotNil(t, opts.ScanConfiguration)
	require.NotNil(t, opts.ScanConfiguration.ScanScheduleHours)
	assert.EqualValues(t, 12, *opts.ScanConfiguration.ScanScheduleHours)
}

func TestIntegrationGcpServerlessGetConfigurationOptions_TagsFilters(t *testing.T) {
	ctx := context.Background()
	tags, diags := types.MapValueFrom(ctx, types.StringType, map[string]string{"env": "production"})
	require.False(t, diags.HasError())
	excluded, diags := types.MapValueFrom(ctx, types.StringType, map[string]string{"env": "sandbox"})
	require.False(t, diags.HasError())

	m := integrationGcpServerlessResourceModel{
		Scope:         types.StringValue("123456789012"),
		HostProjectID: types.StringValue("my-host-project"),
		Region:        types.StringValue("us-central1"),
		ScanConfiguration: &GcpServerlessScanConfigurationInput{
			TagsFilter:         tags,
			ExcludedTagsFilter: excluded,
		},
	}

	opts := m.GetConfigurationOptions()
	require.NotNil(t, opts)
	require.NotNil(t, opts.ScanConfiguration)

	require.NotNil(t, opts.ScanConfiguration.TagsFilter)
	require.Len(t, *opts.ScanConfiguration.TagsFilter, 1)
	kv := (*opts.ScanConfiguration.TagsFilter)[0]
	assert.EqualValues(t, "env", kv.Key)
	require.NotNil(t, kv.Value)
	assert.EqualValues(t, "production", *kv.Value)

	require.NotNil(t, opts.ScanConfiguration.ExcludedTagsFilter)
	require.Len(t, *opts.ScanConfiguration.ExcludedTagsFilter, 1)
	exKv := (*opts.ScanConfiguration.ExcludedTagsFilter)[0]
	assert.EqualValues(t, "env", exKv.Key)
	require.NotNil(t, exKv.Value)
	assert.EqualValues(t, "sandbox", *exKv.Value)
}

func TestIntegrationGcpServerlessGetConfigurationOptions_EmptyTagsFilterOmitted(t *testing.T) {
	m := integrationGcpServerlessResourceModel{
		Scope:         types.StringValue("123456789012"),
		HostProjectID: types.StringValue("my-host-project"),
		Region:        types.StringValue("us-central1"),
		ScanConfiguration: &GcpServerlessScanConfigurationInput{
			TagsFilter:         types.MapNull(types.StringType),
			ExcludedTagsFilter: types.MapNull(types.StringType),
		},
	}

	opts := m.GetConfigurationOptions()
	require.NotNil(t, opts)
	require.NotNil(t, opts.ScanConfiguration)
	assert.Nil(t, opts.ScanConfiguration.TagsFilter)
	assert.Nil(t, opts.ScanConfiguration.ExcludedTagsFilter)
	// No schedule set => the field is omitted from the request.
	assert.Nil(t, opts.ScanConfiguration.ScanScheduleHours)
}
