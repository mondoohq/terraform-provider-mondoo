// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The API reads an empty credentialMrn as set and rejects it next to an
// inline secret ("... and credentialMrn are mutually exclusive"). Every
// integration input must leave credentialMrn out unless the user set it.

func encodeInput(t *testing.T, input any) string {
	t.Helper()
	b, err := json.Marshal(input)
	require.NoError(t, err)
	return string(b)
}

func TestOptionalStringPtr(t *testing.T) {
	assert.Nil(t, OptionalStringPtr(types.StringNull()))
	assert.Nil(t, OptionalStringPtr(types.StringUnknown()))
	v := OptionalStringPtr(types.StringValue("x"))
	require.NotNil(t, v)
	assert.EqualValues(t, "x", *v)
}

func TestIntegrationInputs_CredentialMrnOmittedWhenUnset(t *testing.T) {
	ctx := context.Background()
	targets, diags := types.ListValueFrom(ctx, types.StringType, []string{"mondoo.com"})
	require.False(t, diags.HasError())

	for _, tc := range []struct {
		name  string
		input any
	}{
		{
			name: "okta",
			input: integrationOktaResourceModel{
				CredentialMrn: types.StringNull(),
				Organization:  types.StringValue("an-org"),
				Token:         types.StringValue("a-token"),
			}.GetConfigurationOptions(),
		},
		{
			name: "google workspace",
			input: integrationGoogleWorkspaceResourceModel{
				CredentialMrn:         types.StringNull(),
				CustomerId:            types.StringValue("a-customer"),
				ImpersonatedUserEmail: types.StringValue("admin@example.com"),
				ServiceAccount:        types.StringValue("{}"),
			}.GetConfigurationOptions(),
		},
		{
			name: "crowdstrike",
			input: func() any {
				opts, d := integrationCrowdstrikeResourceModel{
					ClientId:     types.StringValue("a-client-id"),
					ClientSecret: types.StringValue("a-client-secret"),
					FindingTypes: types.SetNull(types.StringType),
					Severities:   types.SetNull(types.StringType),
				}.GetConfigurationOptions(ctx)
				require.False(t, d.HasError())
				return opts
			}(),
		},
		{
			name: "oci",
			input: integrationOciTenantResourceModel{
				Tenancy: types.StringValue("ocid1.tenancy"),
				User:    types.StringValue("ocid1.user"),
				Region:  types.StringValue("us-ashburn-1"),
				Credential: integrationOciCredentialModel{
					Fingerprint: types.StringValue("aa:bb"),
					PrivateKey:  types.StringValue("a-key"),
				},
			}.GetConfigurationOptions(),
		},
		{
			name: "shodan",
			input: integrationShodanResourceModel{
				Targets:     targets,
				Credentials: &integrationShodanCredentialModel{Token: types.StringValue("a-token")},
			}.GetConfigurationOptions(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.NotContains(t, encodeInput(t, tc.input), `"credentialMrn"`)
		})
	}
}

// With a credential reference and no inline secret, the secret must be left
// out rather than sent as "", which the API would count as set.
func TestIntegrationInputs_SecretOmittedWithCredentialMrn(t *testing.T) {
	okta := encodeInput(t, integrationOktaResourceModel{
		CredentialMrn: types.StringValue("//captain.api.mondoo.app/spaces/s/credentials/c"),
		Organization:  types.StringValue("an-org"),
		Token:         types.StringNull(),
	}.GetConfigurationOptions())
	assert.Contains(t, okta, `"credentialMrn":"//captain.api.mondoo.app/spaces/s/credentials/c"`)
	assert.NotContains(t, okta, `"token"`)

	gw := encodeInput(t, integrationGoogleWorkspaceResourceModel{
		CredentialMrn:         types.StringValue("//captain.api.mondoo.app/spaces/s/credentials/c"),
		CustomerId:            types.StringValue("a-customer"),
		ImpersonatedUserEmail: types.StringValue("admin@example.com"),
		ServiceAccount:        types.StringNull(),
	}.GetConfigurationOptions())
	assert.Contains(t, gw, `"credentialMrn"`)
	assert.NotContains(t, gw, `"serviceAccount"`)
}
