// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mondoov1 "go.mondoo.com/mondoo-go"
	"go.mondoo.com/mondoo-go/option"
)

type gqlRequest struct {
	Query     string                     `json:"query"`
	Variables map[string]json.RawMessage `json:"variables"`
}

// fakeAction1API answers every request with response and records what was sent.
func fakeAction1API(t *testing.T, response string) (*ExtendedGqlClient, *[]gqlRequest) {
	t.Helper()
	var requests []gqlRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		var req gqlRequest
		require.NoError(t, json.Unmarshal(body, &req))
		requests = append(requests, req)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(srv.Close)

	client, err := mondoov1.NewClient(option.WithEndpoint(srv.URL), option.WithoutAuthentication())
	require.NoError(t, err)
	return &ExtendedGqlClient{Client: client}, &requests
}

func inputVariable(t *testing.T, req gqlRequest) map[string]any {
	t.Helper()
	var input map[string]any
	require.NoError(t, json.Unmarshal(req.Variables["input"], &input))
	return input
}

func TestAction1CreateCredential(t *testing.T) {
	c, reqs := fakeAction1API(t, `{"data":{"createCredentialV2":{"credential":{"mrn":"//cred"}}}}`)

	mrn, err := c.CreateAction1Credential(context.Background(), "//captain.api.mondoo.app/spaces/s1", "name", "id", "secret", "eu")
	require.NoError(t, err)
	assert.Equal(t, "//cred", mrn)

	require.Len(t, *reqs, 1)
	req := (*reqs)[0]
	assert.Contains(t, req.Query, "$input:CreateCredentialV2Input!")
	assert.Contains(t, req.Query, "createCredentialV2(input: $input)")
	assert.Equal(t, map[string]any{
		"scopeMrn": "//captain.api.mondoo.app/spaces/s1",
		"name":     "name",
		"secret": map[string]any{
			"action1": map[string]any{"clientId": "id", "clientSecret": "secret", "region": "eu"},
		},
	}, inputVariable(t, req))
}

func TestAction1RotateCredential(t *testing.T) {
	c, reqs := fakeAction1API(t, `{"data":{"rotateCredentialV2":{"credential":{"mrn":"//cred"}}}}`)

	require.NoError(t, c.RotateAction1Credential(context.Background(), "//cred", "id2", "secret2", "uk"))

	req := (*reqs)[0]
	assert.Contains(t, req.Query, "$input:RotateCredentialV2Input!")
	assert.Equal(t, map[string]any{
		"mrn": "//cred",
		"secret": map[string]any{
			"action1": map[string]any{"clientId": "id2", "clientSecret": "secret2", "region": "uk"},
		},
	}, inputVariable(t, req))
}

func action1TestModel(scope types.String) integrationAction1ResourceModel {
	return integrationAction1ResourceModel{
		Region:            types.StringValue("eu"),
		OrganizationID:    types.StringValue("org-1"),
		ImportAssetsScope: scope,
	}
}

func TestAction1CreateIntegration(t *testing.T) {
	c, reqs := fakeAction1API(t, `{"data":{"createClientIntegration":{"integration":{"mrn":"//int"}}}}`)

	credential := "//cred"
	created, err := c.CreateIntegration(context.Background(), "//space", "name", mondoov1.ClientIntegrationTypeAction1,
		action1TestModel(types.StringValue("OFF")).configurationOptions(&credential))
	require.NoError(t, err)
	assert.Equal(t, "//int", string(created.Mrn))

	req := (*reqs)[0]
	assert.Contains(t, req.Query, "$input:CreateClientIntegrationInput!")
	assert.Equal(t, map[string]any{
		"spaceMrn":       "//space",
		"name":           "name",
		"type":           "ACTION1",
		"longLivedToken": false,
		"configurationOptions": map[string]any{
			"action1ConfigurationOptions": map[string]any{
				"region":            "eu",
				"organizationId":    "org-1",
				"credentialMrn":     "//cred",
				"importAssetsScope": "OFF",
			},
		},
	}, inputVariable(t, req))
}

func TestAction1UpdateIntegrationKeepsCredentialAndScope(t *testing.T) {
	// A scope with no known value is left out of the request.
	for name, scope := range map[string]types.String{"unknown": types.StringUnknown(), "null": types.StringNull()} {
		t.Run(name, func(t *testing.T) {
			c, reqs := fakeAction1API(t, `{"data":{"updateClientIntegrationConfiguration":{"name":"n"}}}`)

			_, err := c.UpdateIntegration(context.Background(), "//int", "n", mondoov1.ClientIntegrationTypeAction1,
				action1TestModel(scope).configurationOptions(nil))
			require.NoError(t, err)

			req := (*reqs)[0]
			assert.Contains(t, req.Query, "$input:UpdateClientIntegrationConfigurationInput!")
			input := inputVariable(t, req)
			assert.Equal(t, "ACTION1", input["type"])
			// No credentialMrn or importAssetsScope: the API keeps what is stored.
			assert.Equal(t, map[string]any{
				"action1ConfigurationOptions": map[string]any{"region": "eu", "organizationId": "org-1"},
			}, input["configurationOptions"])
		})
	}
}

// ALL is refused at plan time: Action1 has no tenant-wide import.
func TestAction1ImportAssetsScopeValidator(t *testing.T) {
	var resp resource.SchemaResponse
	(&integrationAction1Resource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	attr, ok := resp.Schema.Attributes["import_assets_scope"].(schema.StringAttribute)
	require.True(t, ok)

	for value, wantErr := range map[string]bool{"SCANNED_GROUPS": false, "OFF": false, "ALL": true, "off": true} {
		var res validator.StringResponse
		for _, v := range attr.Validators {
			v.ValidateString(context.Background(), validator.StringRequest{ConfigValue: types.StringValue(value)}, &res)
		}
		assert.Equal(t, wantErr, res.Diagnostics.HasError(), value)
	}
}

// action1Update runs the resource's Update from state to plan and returns the
// mutations it sent, in order.
func action1Update(t *testing.T, state, plan integrationAction1ResourceModel) ([]gqlRequest, integrationAction1ResourceModel) {
	t.Helper()
	ctx := context.Background()
	var s resource.SchemaResponse
	r := &integrationAction1Resource{}
	r.Schema(ctx, resource.SchemaRequest{}, &s)

	c, reqs := fakeAction1API(t, `{"data":{}}`)
	r.client = c

	req := resource.UpdateRequest{Plan: tfsdk.Plan{Schema: s.Schema}, State: tfsdk.State{Schema: s.Schema}}
	req.Plan.Raw = tftypes.NewValue(s.Schema.Type().TerraformType(ctx), nil)
	req.State.Raw = req.Plan.Raw
	require.False(t, req.Plan.Set(ctx, &plan).HasError())
	require.False(t, req.State.Set(ctx, &state).HasError())

	resp := resource.UpdateResponse{State: tfsdk.State{Schema: s.Schema, Raw: req.Plan.Raw}}
	r.Update(ctx, req, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)

	var got integrationAction1ResourceModel
	require.False(t, resp.State.Get(ctx, &got).HasError())
	return *reqs, got
}

func action1ManagedModel() integrationAction1ResourceModel {
	m := action1TestModel(types.StringValue("SCANNED_GROUPS"))
	m.SpaceID = types.StringValue("s1")
	m.Mrn = types.StringValue("//int")
	m.Name = types.StringValue("n")
	m.ClientID = types.StringValue("id")
	m.ClientSecret = types.StringValue("secret")
	m.ManagedCredentialMrn = types.StringValue("//cred")
	return m
}

// The managed credential holds the region as well, so moving the integration to
// another region has to move the credential with it, before the integration.
func TestAction1UpdateRegionRotatesManagedCredential(t *testing.T) {
	plan := action1ManagedModel()
	plan.Region = types.StringValue("uk")

	reqs, _ := action1Update(t, action1ManagedModel(), plan)

	require.Len(t, reqs, 2)
	assert.Contains(t, reqs[0].Query, "rotateCredentialV2")
	assert.Equal(t, map[string]any{
		"mrn": "//cred",
		"secret": map[string]any{
			"action1": map[string]any{"clientId": "id", "clientSecret": "secret", "region": "uk"},
		},
	}, inputVariable(t, reqs[0]))
	assert.Contains(t, reqs[1].Query, "updateClientIntegrationConfiguration")
}

func TestAction1UpdateRenameKeepsManagedCredential(t *testing.T) {
	plan := action1ManagedModel()
	plan.Name = types.StringValue("renamed")

	reqs, got := action1Update(t, action1ManagedModel(), plan)

	require.Len(t, reqs, 1)
	assert.Contains(t, reqs[0].Query, "updateClientIntegrationConfiguration")
	assert.Equal(t, "//cred", got.ManagedCredentialMrn.ValueString())
	assert.Equal(t, "SCANNED_GROUPS", got.ImportAssetsScope.ValueString())
}

func TestAction1GetIntegration(t *testing.T) {
	c, reqs := fakeAction1API(t, `{"data":{"clientIntegration":{"integration":{
		"mrn":"//integration.api.mondoo.app/spaces/s1/integrations/i1","name":"n",
		"configurationOptions":{"region":"uk","organizationId":"org-1","importAssetsScope":"SCANNED_GROUPS"}}}}}`)

	integration, err := c.GetAction1Integration(context.Background(), "//integration.api.mondoo.app/spaces/s1/integrations/i1")
	require.NoError(t, err)
	assert.Contains(t, (*reqs)[0].Query, "... on Action1ConfigurationOptions{region,organizationId,importAssetsScope}")

	var m integrationAction1ResourceModel
	m.applyIntegration(integration)
	assert.Equal(t, "s1", m.SpaceID.ValueString())
	assert.Equal(t, "n", m.Name.ValueString())
	assert.Equal(t, "uk", m.Region.ValueString())
	assert.Equal(t, "org-1", m.OrganizationID.ValueString())
	assert.Equal(t, "SCANNED_GROUPS", m.ImportAssetsScope.ValueString())
}

func TestAction1DeleteCredential(t *testing.T) {
	c, reqs := fakeAction1API(t, `{"data":{"deleteCredentialV2":{"deletedMrn":"//cred"}}}`)

	require.NoError(t, c.DeleteCredentialV2(context.Background(), "//cred"))
	assert.Contains(t, (*reqs)[0].Query, "$input:DeleteCredentialV2Input!")
	assert.Equal(t, map[string]any{"mrn": "//cred"}, inputVariable(t, (*reqs)[0]))
}

func TestAction1FillStoredImportAssetsScope(t *testing.T) {
	t.Run("known value needs no read", func(t *testing.T) {
		c, reqs := fakeAction1API(t, `{}`)
		r := &integrationAction1Resource{client: c}
		data := action1TestModel(types.StringValue("OFF"))
		var diags diag.Diagnostics

		r.fillStoredImportAssetsScope(context.Background(), &data, &diags)

		assert.Empty(t, *reqs)
		assert.Equal(t, "OFF", data.ImportAssetsScope.ValueString())
		assert.False(t, diags.HasError())
	})

	t.Run("unknown is read from the API", func(t *testing.T) {
		c, _ := fakeAction1API(t, `{"data":{"clientIntegration":{"integration":{
			"mrn":"//int","name":"n","configurationOptions":{"importAssetsScope":"OFF"}}}}}`)
		r := &integrationAction1Resource{client: c}
		data := action1TestModel(types.StringUnknown())
		var diags diag.Diagnostics

		r.fillStoredImportAssetsScope(context.Background(), &data, &diags)

		assert.Equal(t, "OFF", data.ImportAssetsScope.ValueString())
		assert.Zero(t, diags.WarningsCount())
	})

	t.Run("a failed read warns and records the default", func(t *testing.T) {
		c, _ := fakeAction1API(t, `{"errors":[{"message":"unavailable"}]}`)
		r := &integrationAction1Resource{client: c}
		data := action1TestModel(types.StringUnknown())
		var diags diag.Diagnostics

		r.fillStoredImportAssetsScope(context.Background(), &data, &diags)

		assert.Equal(t, "SCANNED_GROUPS", data.ImportAssetsScope.ValueString())
		assert.Equal(t, 1, diags.WarningsCount())
		assert.False(t, diags.HasError())
	})
}
