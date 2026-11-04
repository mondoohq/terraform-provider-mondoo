// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// orgScopedIntegrationResources are the hosted scan integrations that can be
// created in an organization, each with an empty model to decode state into.
var orgScopedIntegrationResources = map[string]struct {
	new   func() resource.Resource
	model any
}{
	"aws":              {NewIntegrationAwsResource, &integrationAwsResourceModel{}},
	"azure":            {NewIntegrationAzureResource, &integrationAzureResourceModel{}},
	"gcp":              {NewIntegrationGcpResource, &integrationGcpResourceModel{}},
	"ms365":            {NewIntegrationMs365Resource, &integrationMs365ResourceModel{}},
	"okta":             {NewIntegrationOktaResource, &integrationOktaResourceModel{}},
	"google_workspace": {NewIntegrationGoogleWorkspaceResource, &integrationGoogleWorkspaceResourceModel{}},
	"slack":            {NewIntegrationSlackResource, &integrationSlackResourceModel{}},
	"gitlab":           {NewIntegrationGitlabResource, &integrationGitlabResourceModel{}},
	"shodan":           {NewIntegrationShodanResource, &integrationShodanResourceModel{}},
	"domain":           {NewIntegrationDomainResource, &integrationDomainResourceModel{}},
	"github":           {NewIntegrationGithubResource, &integrationGithubResourceModel{}},
	"oci_tenant":       {NewIntegrationOciTenantResource, &integrationOciTenantResourceModel{}},
}

func TestOrgScopedIntegrationSchemas(t *testing.T) {
	ctx := context.Background()
	for name, tc := range orgScopedIntegrationResources {
		t.Run(name, func(t *testing.T) {
			resp := &resource.SchemaResponse{}
			tc.new().Schema(ctx, resource.SchemaRequest{}, resp)
			require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
			require.False(t, resp.Schema.ValidateImplementation(ctx).HasError())
			assert.Contains(t, resp.Schema.MarkdownDescription, orgScopedIntegrationDescription)

			orgID, ok := resp.Schema.Attributes["org_id"].(schema.StringAttribute)
			require.True(t, ok, "org_id is a string attribute")
			assert.True(t, orgID.Optional)
			assert.False(t, orgID.Computed)
			require.Len(t, orgID.Validators, 1)
			assert.Contains(t, orgID.Validators[0].Description(ctx), "space_id", "org_id conflicts with space_id")
			require.Len(t, orgID.PlanModifiers, 1)
			assert.Contains(t, orgID.PlanModifiers[0].Description(ctx), "destroy and recreate")

			spaceID, ok := resp.Schema.Attributes["space_id"].(schema.StringAttribute)
			require.True(t, ok, "space_id is a string attribute")
			assert.True(t, spaceID.Optional)
			assert.True(t, spaceID.Computed)
			require.Len(t, spaceID.PlanModifiers, 1, "UseStateForUnknown only, as before org_id existed")

			// The model decodes state that holds org_id: the field exists and
			// matches the schema.
			attrs := map[string]tftypes.Value{}
			objType := resp.Schema.Type().TerraformType(ctx).(tftypes.Object)
			for attr, typ := range objType.AttributeTypes {
				attrs[attr] = nullAttributes(typ)
			}
			attrs["org_id"] = tftypes.NewValue(tftypes.String, "my-org")
			state := tfsdk.State{Schema: resp.Schema, Raw: tftypes.NewValue(objType, attrs)}
			model := reflect.New(reflect.TypeOf(tc.model).Elem()).Interface()
			diags := state.Get(ctx, model)
			require.False(t, diags.HasError(), diags)
			got := reflect.ValueOf(model).Elem().FieldByName("OrgID").Interface().(types.String)
			assert.Equal(t, "my-org", got.ValueString())
		})
	}
}

// nullAttributes is a null value, except that an object is an object of null
// attributes: some models hold a nested block as a struct, not a pointer.
func nullAttributes(typ tftypes.Type) tftypes.Value {
	obj, ok := typ.(tftypes.Object)
	if !ok {
		return tftypes.NewValue(typ, nil)
	}
	attrs := map[string]tftypes.Value{}
	for attr, t := range obj.AttributeTypes {
		attrs[attr] = tftypes.NewValue(t, nil)
	}
	return tftypes.NewValue(obj, attrs)
}

func TestIntegrationScope(t *testing.T) {
	tests := []struct {
		name    string
		mrn     string
		orgID   types.String
		spaceID types.String
	}{
		{
			name:    "organization",
			mrn:     "//integration.api.mondoo.app/organizations/my-org/integrations/abc",
			orgID:   types.StringValue("my-org"),
			spaceID: types.StringNull(),
		},
		{
			name:    "space",
			mrn:     "//integration.api.mondoo.app/spaces/my-space/integrations/abc",
			orgID:   types.StringNull(),
			spaceID: types.StringValue("my-space"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orgID, spaceID := integrationScope(Integration{Mrn: tt.mrn})
			assert.Equal(t, tt.orgID, orgID)
			assert.Equal(t, tt.spaceID, spaceID)
		})
	}
}

func TestIntegrationScopeMrn(t *testing.T) {
	client := &ExtendedGqlClient{space: "provider-space"}
	tests := []struct {
		name     string
		orgID    types.String
		spaceID  types.String
		client   *ExtendedGqlClient
		scopeMrn string
		stored   types.String
		wantErr  bool
	}{
		{
			name:     "org_id creates in the organization and leaves space_id null",
			orgID:    types.StringValue("my-org"),
			spaceID:  types.StringUnknown(),
			client:   client,
			scopeMrn: "//captain.api.mondoo.app/organizations/my-org",
			stored:   types.StringNull(),
		},
		{
			name:     "space_id",
			orgID:    types.StringNull(),
			spaceID:  types.StringValue("my-space"),
			client:   client,
			scopeMrn: "//captain.api.mondoo.app/spaces/my-space",
			stored:   types.StringValue("my-space"),
		},
		{
			name:     "provider space",
			orgID:    types.StringNull(),
			spaceID:  types.StringUnknown(),
			client:   client,
			scopeMrn: "//captain.api.mondoo.app/spaces/provider-space",
			stored:   types.StringValue("provider-space"),
		},
		{
			name:    "no scope anywhere",
			orgID:   types.StringNull(),
			spaceID: types.StringUnknown(),
			client:  &ExtendedGqlClient{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopeMrn, stored, err := integrationScopeMrn(tt.client, tt.orgID, tt.spaceID)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.scopeMrn, scopeMrn)
			assert.Equal(t, tt.stored, stored)
		})
	}
}

// The domain resource rebuilds its state from the API on every read, so an
// organization integration must not put the organization id into space_id.
func TestDomainModelFrom(t *testing.T) {
	integration := Integration{
		Mrn: "//integration.api.mondoo.app/organizations/my-org/integrations/abc",
	}
	integration.ConfigurationOptions.HostConfigurationOptions.Host = "mondoo.com"
	integration.ConfigurationOptions.HostConfigurationOptions.HTTPS = true

	m := domainModelFrom(integration)
	assert.Equal(t, "my-org", m.OrgID.ValueString())
	assert.True(t, m.SpaceID.IsNull())
	assert.Equal(t, "mondoo.com", m.Host.ValueString())
	assert.True(t, m.Https.ValueBool())
	assert.False(t, m.Http.ValueBool())

	integration.Mrn = "//integration.api.mondoo.app/spaces/my-space/integrations/abc"
	m = domainModelFrom(integration)
	assert.True(t, m.OrgID.IsNull())
	assert.Equal(t, "my-space", m.SpaceID.ValueString())
	assert.True(t, strings.HasSuffix(m.Mrn.ValueString(), "/integrations/abc"))
}
