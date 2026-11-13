// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// action1Regions are the Action1 regions the API accepts.
var action1Regions = []string{"na", "na-2", "eu", "uk", "au"}

// action1ImportAssetsScopes are the import_assets_scope values an Action1
// integration accepts. The API refuses ALL: Action1 has no tenant-wide import.
var action1ImportAssetsScopes = []string{
	string(mondoov1.ImportAssetsScopeScannedGroups),
	string(mondoov1.ImportAssetsScopeOff),
}

// Action1ConfigurationOptions is the stored configuration of an Action1 integration.
type Action1ConfigurationOptions struct {
	Region            string
	OrganizationId    string
	ImportAssetsScope string
}

type action1Integration struct {
	Mrn                  string
	Name                 string
	ConfigurationOptions struct {
		Action1ConfigurationOptions Action1ConfigurationOptions `graphql:"... on Action1ConfigurationOptions"`
	} `graphql:"configurationOptions"`
}

// GetAction1Integration reads an Action1 integration. It has its own query so
// that the query the other integration resources share does not depend on the
// Action1 fields.
func (c *ExtendedGqlClient) GetAction1Integration(ctx context.Context, mrn string) (action1Integration, error) {
	var q struct {
		ClientIntegration struct {
			Integration action1Integration
		} `graphql:"clientIntegration(input: {mrn: $mrn})"`
	}
	variables := map[string]interface{}{
		"mrn": mondoov1.String(mrn),
	}
	if err := c.Query(ctx, &q, variables); err != nil {
		return action1Integration{}, err
	}
	return q.ClientIntegration.Integration, nil
}

// action1CredentialSecret is the payload of an Action1 credential. A rotation
// replaces the whole payload, so it always carries the region: the credential's
// health check asks that region's token endpoint, and without a region it fails.
func action1CredentialSecret(clientID, clientSecret, region string) mondoov1.CredentialV2SecretInput {
	return mondoov1.CredentialV2SecretInput{
		Action1: &mondoov1.Action1CredentialV2Input{
			ClientId:     mondoov1.String(clientID),
			ClientSecret: mondoov1.String(clientSecret),
			Region:       mondoov1.NewStringPtr(mondoov1.String(region)),
		},
	}
}

// CreateAction1Credential stores an Action1 API client as a typed credential in
// the given scope and returns its MRN.
func (c *ExtendedGqlClient) CreateAction1Credential(ctx context.Context, scopeMrn, name, clientID, clientSecret, region string) (string, error) {
	var m struct {
		CreateCredentialV2 struct {
			Credential struct {
				Mrn string
			}
		} `graphql:"createCredentialV2(input: $input)"`
	}
	input := mondoov1.CreateCredentialV2Input{
		ScopeMrn: mondoov1.ID(scopeMrn),
		Name:     mondoov1.String(name),
		Secret:   action1CredentialSecret(clientID, clientSecret, region),
	}
	tflog.Trace(ctx, "CreateCredentialV2", map[string]interface{}{"scope": scopeMrn, "name": name})
	if err := c.Mutate(ctx, &m, input, nil); err != nil {
		return "", err
	}
	return m.CreateCredentialV2.Credential.Mrn, nil
}

// RotateAction1Credential replaces the API client and region stored in an Action1 typed credential.
func (c *ExtendedGqlClient) RotateAction1Credential(ctx context.Context, mrn, clientID, clientSecret, region string) error {
	var m struct {
		RotateCredentialV2 struct {
			Credential struct {
				Mrn string
			}
		} `graphql:"rotateCredentialV2(input: $input)"`
	}
	input := mondoov1.RotateCredentialV2Input{
		Mrn:    mondoov1.ID(mrn),
		Secret: action1CredentialSecret(clientID, clientSecret, region),
	}
	tflog.Trace(ctx, "RotateCredentialV2", map[string]interface{}{"mrn": mrn})
	return c.Mutate(ctx, &m, input, nil)
}

func (c *ExtendedGqlClient) DeleteCredentialV2(ctx context.Context, mrn string) error {
	var m struct {
		DeleteCredentialV2 struct {
			DeletedMrn string
		} `graphql:"deleteCredentialV2(input: $input)"`
	}
	input := mondoov1.DeleteCredentialV2Input{Mrn: mondoov1.ID(mrn)}
	tflog.Trace(ctx, "DeleteCredentialV2", map[string]interface{}{"mrn": mrn})
	return c.Mutate(ctx, &m, input, nil)
}
