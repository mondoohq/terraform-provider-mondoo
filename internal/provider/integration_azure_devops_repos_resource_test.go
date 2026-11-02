// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mondoov1 "go.mondoo.com/mondoo-go"
)

func TestAzureDevopsReposSchemaIsValid(t *testing.T) {
	ctx := context.Background()
	resp := &resource.SchemaResponse{}
	NewIntegrationAzureDevopsReposResource().Schema(ctx, resource.SchemaRequest{}, resp)
	require.False(t, resp.Diagnostics.HasError(), resp.Diagnostics)
	require.False(t, resp.Schema.ValidateImplementation(ctx).HasError())
}

func adoReposModel(secret, cert types.String) integrationAzureDevopsReposResourceModel {
	return integrationAzureDevopsReposResourceModel{
		Organization:        types.StringValue("acme"),
		Project:             types.StringNull(),
		Repository:          types.StringNull(),
		RepositoryAllowList: types.ListNull(types.StringType),
		RepositoryDenyList:  types.ListNull(types.StringType),
		SecretScan:          types.BoolValue(true),
		Credentials: &azureDevopsReposCredentialsModel{
			TenantID:     types.StringValue("tenant"),
			ClientID:     types.StringValue("client"),
			ClientSecret: secret,
			Certificate:  cert,
		},
	}
}

func TestAzureDevopsReposConfigInput(t *testing.T) {
	t.Run("client secret", func(t *testing.T) {
		opts := adoReposModel(types.StringValue("s3cret"), types.StringNull()).configInput(true).AzureDevopsReposConfigurationOptions
		require.NotNil(t, opts.EntraClientSecret)
		assert.Equal(t, mondoov1.AzureDevopsReposCredentialModeEntraClientSecret, *opts.CredentialMode)
		assert.Equal(t, mondoov1.String("s3cret"), *opts.EntraClientSecret.ClientSecret)
		assert.Nil(t, opts.EntraCertificate)
		assert.Nil(t, opts.Project, "no project means the whole organization")
		assert.True(t, bool(*opts.EnableSecretScan))
		assert.False(t, bool(*opts.EnableCodeScan), "an unset toggle is sent as off")
	})

	t.Run("certificate", func(t *testing.T) {
		opts := adoReposModel(types.StringNull(), types.StringValue("PEM")).configInput(true).AzureDevopsReposConfigurationOptions
		require.NotNil(t, opts.EntraCertificate)
		assert.Equal(t, mondoov1.AzureDevopsReposCredentialModeEntraCertificate, *opts.CredentialMode)
		assert.Equal(t, mondoov1.String("PEM"), *opts.EntraCertificate.Certificate)
		assert.Nil(t, opts.EntraClientSecret)
	})

	// The API needs the mode on every update, but keeps the stored credential
	// when the arm is left out.
	t.Run("unchanged credential", func(t *testing.T) {
		opts := adoReposModel(types.StringValue("s3cret"), types.StringNull()).configInput(false).AzureDevopsReposConfigurationOptions
		assert.Equal(t, mondoov1.AzureDevopsReposCredentialModeEntraClientSecret, *opts.CredentialMode)
		assert.Nil(t, opts.EntraClientSecret)
		assert.Nil(t, opts.EntraCertificate)
	})

	t.Run("one repository", func(t *testing.T) {
		m := adoReposModel(types.StringValue("s3cret"), types.StringNull())
		m.Project, m.Repository = types.StringValue("web"), types.StringValue("api")
		opts := m.configInput(true).AzureDevopsReposConfigurationOptions
		assert.Equal(t, mondoov1.String("web"), *opts.Project)
		assert.Equal(t, mondoov1.String("api"), *opts.Repository)
	})
}

func TestAzureDevopsReposModelFrom(t *testing.T) {
	integration := Integration{
		Mrn:  "//integration.api.mondoo.app/organizations/my-org/integrations/abc",
		Name: "ado",
	}
	o := &integration.ConfigurationOptions.AzureDevopsReposConfigurationOptions
	o.Organization = "Acme"
	o.CredentialMode = string(mondoov1.AzureDevopsReposCredentialModeEntraCertificate)
	o.EntraCertificate.TenantId = "tenant"
	o.EntraCertificate.ClientId = "client"
	o.ReposDenyList = []string{"archive-*"}
	o.EnableVulnScan = true

	t.Run("import", func(t *testing.T) {
		m := azureDevopsReposModelFrom(integration, nil)
		assert.Equal(t, "my-org", m.OrgID.ValueString())
		assert.True(t, m.SpaceID.IsNull())
		assert.Equal(t, "Acme", m.Organization.ValueString())
		assert.True(t, m.Project.IsNull())
		assert.Equal(t, "tenant", m.Credentials.TenantID.ValueString())
		assert.Equal(t, "client", m.Credentials.ClientID.ValueString())
		assert.True(t, m.Credentials.Certificate.IsNull(), "the API never returns the certificate")
		assert.True(t, m.RepositoryAllowList.IsNull(), "an empty list reads as unset, not as drift")
		assert.Len(t, m.RepositoryDenyList.Elements(), 1)
		assert.True(t, m.VulnerabilityScan.ValueBool())
	})

	t.Run("refresh keeps the secret and the user's casing", func(t *testing.T) {
		prior := adoReposModel(types.StringNull(), types.StringValue("PEM"))
		m := azureDevopsReposModelFrom(integration, &prior)
		assert.Equal(t, "acme", m.Organization.ValueString())
		assert.Equal(t, "PEM", m.Credentials.Certificate.ValueString())
	})

	t.Run("space scope", func(t *testing.T) {
		spaced := integration
		spaced.Mrn = "//integration.api.mondoo.app/spaces/my-space/integrations/abc"
		m := azureDevopsReposModelFrom(spaced, nil)
		assert.Equal(t, "my-space", m.SpaceID.ValueString())
		assert.True(t, m.OrgID.IsNull())
	})
}
