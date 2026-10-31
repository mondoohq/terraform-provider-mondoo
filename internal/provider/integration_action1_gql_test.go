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

	mrn, err := c.CreateAction1Credential(context.Background(), "//captain.api.mondoo.app/spaces/s1", "name", "id", "secret")
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
			"action1": map[string]any{"clientId": "id", "clientSecret": "secret"},
		},
	}, inputVariable(t, req))
}

func TestAction1RotateCredential(t *testing.T) {
	c, reqs := fakeAction1API(t, `{"data":{"rotateCredentialV2":{"credential":{"mrn":"//cred"}}}}`)

	require.NoError(t, c.RotateAction1Credential(context.Background(), "//cred", "id2", "secret2"))

	req := (*reqs)[0]
	assert.Contains(t, req.Query, "$input:RotateCredentialV2Input!")
	assert.Equal(t, map[string]any{
		"mrn": "//cred",
		"secret": map[string]any{
			"action1": map[string]any{"clientId": "id2", "clientSecret": "secret2"},
		},
	}, inputVariable(t, req))
}

func TestAction1CreateIntegration(t *testing.T) {
	c, reqs := fakeAction1API(t, `{"data":{"createClientIntegration":{"integration":{"mrn":"//int"}}}}`)

	mrn, err := c.CreateAction1Integration(context.Background(), "//space", "name", "eu", "org-1", "//cred")
	require.NoError(t, err)
	assert.Equal(t, "//int", mrn)

	req := (*reqs)[0]
	assert.Contains(t, req.Query, "$input:CreateClientIntegrationInput!")
	assert.Equal(t, map[string]any{
		"spaceMrn":       "//space",
		"name":           "name",
		"type":           "ACTION1",
		"longLivedToken": false,
		"configurationOptions": map[string]any{
			"action1ConfigurationOptions": map[string]any{
				"region":         "eu",
				"organizationId": "org-1",
				"credentialMrn":  "//cred",
			},
		},
	}, inputVariable(t, req))
}

func TestAction1UpdateIntegrationKeepsCredential(t *testing.T) {
	c, reqs := fakeAction1API(t, `{"data":{"updateClientIntegrationConfiguration":{"name":"n"}}}`)

	require.NoError(t, c.UpdateAction1Integration(context.Background(), "//int", "n", "na-2", "org-1", nil))

	req := (*reqs)[0]
	assert.Contains(t, req.Query, "$input:UpdateClientIntegrationConfigurationInput!")
	input := inputVariable(t, req)
	assert.Equal(t, "ACTION1", input["type"])
	// No credentialMrn: the API keeps the integration's current credential.
	assert.Equal(t, map[string]any{
		"action1ConfigurationOptions": map[string]any{"region": "na-2", "organizationId": "org-1"},
	}, input["configurationOptions"])
}

func TestAction1GetIntegration(t *testing.T) {
	c, reqs := fakeAction1API(t, `{"data":{"clientIntegration":{"integration":{
		"mrn":"//integration.api.mondoo.app/spaces/s1/integrations/i1","name":"n",
		"configurationOptions":{"region":"uk","organizationId":"org-1"}}}}}`)

	integration, err := c.GetAction1Integration(context.Background(), "//integration.api.mondoo.app/spaces/s1/integrations/i1")
	require.NoError(t, err)
	assert.Contains(t, (*reqs)[0].Query, "... on Action1ConfigurationOptions{region,organizationId}")

	var m integrationAction1ResourceModel
	m.applyIntegration(integration)
	assert.Equal(t, "s1", m.SpaceID.ValueString())
	assert.Equal(t, "n", m.Name.ValueString())
	assert.Equal(t, "uk", m.Region.ValueString())
	assert.Equal(t, "org-1", m.OrganizationID.ValueString())
}

func TestAction1DeleteCredential(t *testing.T) {
	c, reqs := fakeAction1API(t, `{"data":{"deleteCredentialV2":{"deletedMrn":"//cred"}}}`)

	require.NoError(t, c.DeleteCredentialV2(context.Background(), "//cred"))
	assert.Contains(t, (*reqs)[0].Query, "$input:DeleteCredentialV2Input!")
	assert.Equal(t, map[string]any{"mrn": "//cred"}, inputVariable(t, (*reqs)[0]))
}
