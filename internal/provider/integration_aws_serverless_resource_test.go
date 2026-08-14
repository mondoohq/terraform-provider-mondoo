// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
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
