// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
)

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
