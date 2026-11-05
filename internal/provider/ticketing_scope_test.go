// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The ticketing resources that can be created in an organization, and the
// name of each one's auto-create attribute.
var ticketingOrgResources = []struct {
	name           string
	resource       func() resource.Resource
	autoCreateAttr string
}{
	{"jira", NewIntegrationJiraResource, "auto_create"},
	{"email", NewIntegrationEmailResource, "auto_create"},
	{"zendesk", NewIntegrationZendeskResource, "auto_create"},
	{"azure_devops", NewIntegrationAzureDevopsResource, "auto_create_tickets"},
}

// ticketingTestConfig is a raw configuration of the resource with every
// attribute null except org_id, space_id and the auto-create attribute.
func ticketingTestConfig(t *testing.T, r resource.Resource, autoCreateAttr string, orgID, spaceID, autoCreate interface{}) tfsdk.Config {
	t.Helper()
	ctx := context.Background()
	resp := &resource.SchemaResponse{}
	r.Schema(ctx, resource.SchemaRequest{}, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	objectType, ok := resp.Schema.Type().TerraformType(ctx).(tftypes.Object)
	require.True(t, ok)

	values := map[string]tftypes.Value{}
	for name, typ := range objectType.AttributeTypes {
		values[name] = tftypes.NewValue(typ, nil)
	}
	values["org_id"] = tftypes.NewValue(tftypes.String, orgID)
	values["space_id"] = tftypes.NewValue(tftypes.String, spaceID)
	values[autoCreateAttr] = tftypes.NewValue(tftypes.Bool, autoCreate)
	return tfsdk.Config{Schema: resp.Schema, Raw: tftypes.NewValue(objectType, values)}
}

func TestTicketingOrgSchemasAreValid(t *testing.T) {
	ctx := context.Background()
	for _, tr := range ticketingOrgResources {
		t.Run(tr.name, func(t *testing.T) {
			resp := &resource.SchemaResponse{}
			tr.resource().Schema(ctx, resource.SchemaRequest{}, resp)
			require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
			require.False(t, resp.Schema.ValidateImplementation(ctx).HasError())
			assert.Contains(t, resp.Schema.MarkdownDescription, orgTicketingIntegrationDescription)

			// The same org_id and space_id as the org-scoped scan integrations.
			orgID, ok := resp.Schema.Attributes["org_id"].(schema.StringAttribute)
			require.True(t, ok, "org_id is a string attribute")
			assert.True(t, orgID.Optional)
			require.Len(t, orgID.Validators, 1)
			assert.Contains(t, orgID.Validators[0].Description(ctx), "space_id", "org_id conflicts with space_id")
			require.Len(t, orgID.PlanModifiers, 1)
			assert.Contains(t, orgID.PlanModifiers[0].Description(ctx), "destroy and recreate")

			spaceID, ok := resp.Schema.Attributes["space_id"].(schema.StringAttribute)
			require.True(t, ok, "space_id is a string attribute")
			assert.True(t, spaceID.Optional)
			assert.True(t, spaceID.Computed)
			require.Len(t, spaceID.PlanModifiers, 1, "UseStateForUnknown only, as before org_id existed")
		})
	}
}

// Auto-create is refused at organization scope, as the server refuses it;
// a space integration may still set it.
func TestTicketingOrgRefusesAutoCreate(t *testing.T) {
	cases := []struct {
		name                       string
		orgID, spaceID, autoCreate interface{}
		wantErr                    bool
	}{
		{name: "organization with auto-create", orgID: "my-org", autoCreate: true, wantErr: true},
		{name: "organization without auto-create", orgID: "my-org", autoCreate: false},
		{name: "organization, auto-create unset", orgID: "my-org"},
		{name: "space with auto-create", spaceID: "my-space", autoCreate: true},
		{name: "provider space with auto-create", autoCreate: true},
		{name: "unknown organization is checked at apply", orgID: tftypes.UnknownValue, autoCreate: true},
	}
	for _, tr := range ticketingOrgResources {
		for _, tc := range cases {
			t.Run(tr.name+"/"+tc.name, func(t *testing.T) {
				r := tr.resource()
				withValidators, ok := r.(resource.ResourceWithConfigValidators)
				require.True(t, ok)
				req := resource.ValidateConfigRequest{
					Config: ticketingTestConfig(t, r, tr.autoCreateAttr, tc.orgID, tc.spaceID, tc.autoCreate),
				}
				var resp resource.ValidateConfigResponse
				for _, v := range withValidators.ConfigValidators(context.Background()) {
					if _, isOrg := v.(orgAutoCreateValidator); !isOrg {
						// Other validators (e.g. Azure DevOps credentials) are not under test.
						continue
					}
					v.ValidateResource(context.Background(), req, &resp)
				}
				assert.Equal(t, tc.wantErr, resp.Diagnostics.HasError(), "diagnostics: %v", resp.Diagnostics)
			})
		}
	}
}
