// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationGcpServerlessGetConfigurationOptions_Minimal(t *testing.T) {
	m := integrationGcpServerlessResourceModel{
		Scope:         types.StringValue("123456789012"),
		HostProjectID: types.StringValue("my-host-project"),
		Region:        types.StringValue("us-central1"),
	}

	opts := m.GetConfigurationOptions()
	require.NotNil(t, opts)
	assert.EqualValues(t, "123456789012", opts.Scope)
	assert.EqualValues(t, "my-host-project", opts.HostProjectId)
	assert.EqualValues(t, "us-central1", opts.Region)
	// No scan_configuration block => no scan configuration sent.
	assert.Nil(t, opts.ScanConfiguration)
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
}
