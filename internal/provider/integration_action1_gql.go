// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// GraphQL types for the Action1 integration.
//
// mondoo-go does not have the Action1 types yet, so the inputs below mirror the
// API's input types with only the fields this resource sends. The GraphQL client
// names a variable's type after its Go type, so each input keeps the API's type
// name. Once mondoo-go ships the Action1 types, these can be replaced by the
// generated ones and the generic integration and credential helpers.

// clientIntegrationTypeAction1 is the integration type of an Action1 integration.
const clientIntegrationTypeAction1 mondoov1.ClientIntegrationType = "ACTION1"

// action1Regions are the Action1 regions the API accepts.
var action1Regions = []string{"na", "na-2", "eu", "uk", "au"}

// Action1ConfigurationOptionsInput configures an Action1 integration.
type Action1ConfigurationOptionsInput struct {
	Region         string `json:"region"`
	OrganizationId string `json:"organizationId"`
	// CredentialMrn is the Action1 typed credential. Required on create; on
	// update, omitting it keeps the current one.
	CredentialMrn *string `json:"credentialMrn,omitempty"`
}

type action1ClientIntegrationConfigurationInput struct {
	Action1ConfigurationOptions *Action1ConfigurationOptionsInput `json:"action1ConfigurationOptions,omitempty"`
}

// CreateClientIntegrationInput mirrors the API input of the same name.
type CreateClientIntegrationInput struct {
	SpaceMrn             string                                     `json:"spaceMrn"`
	Name                 string                                     `json:"name"`
	Type                 mondoov1.ClientIntegrationType             `json:"type"`
	LongLivedToken       bool                                       `json:"longLivedToken"`
	ConfigurationOptions action1ClientIntegrationConfigurationInput `json:"configurationOptions"`
}

// UpdateClientIntegrationConfigurationInput mirrors the API input of the same name.
type UpdateClientIntegrationConfigurationInput struct {
	Mrn                  string                                     `json:"mrn"`
	Name                 string                                     `json:"name"`
	Type                 mondoov1.ClientIntegrationType             `json:"type"`
	ConfigurationOptions action1ClientIntegrationConfigurationInput `json:"configurationOptions"`
}

// Action1CredentialV2Input is an Action1 API client.
type Action1CredentialV2Input struct {
	ClientId     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

type action1CredentialV2SecretInput struct {
	Action1 *Action1CredentialV2Input `json:"action1,omitempty"`
}

// CreateCredentialV2Input mirrors the API input of the same name.
type CreateCredentialV2Input struct {
	ScopeMrn string                         `json:"scopeMrn"`
	Name     string                         `json:"name"`
	Secret   action1CredentialV2SecretInput `json:"secret"`
}

// RotateCredentialV2Input mirrors the API input of the same name.
type RotateCredentialV2Input struct {
	Mrn    string                         `json:"mrn"`
	Secret action1CredentialV2SecretInput `json:"secret"`
}

// Action1ConfigurationOptions is the stored configuration of an Action1 integration.
type Action1ConfigurationOptions struct {
	Region         string
	OrganizationId string
}

type action1Integration struct {
	Mrn                  string
	Name                 string
	ConfigurationOptions struct {
		Action1ConfigurationOptions Action1ConfigurationOptions `graphql:"... on Action1ConfigurationOptions"`
	} `graphql:"configurationOptions"`
}

func action1Options(region, organizationID string, credentialMrn *string) action1ClientIntegrationConfigurationInput {
	return action1ClientIntegrationConfigurationInput{
		Action1ConfigurationOptions: &Action1ConfigurationOptionsInput{
			Region:         region,
			OrganizationId: organizationID,
			CredentialMrn:  credentialMrn,
		},
	}
}

func (c *ExtendedGqlClient) CreateAction1Integration(ctx context.Context, spaceMrn, name, region, organizationID, credentialMrn string) (string, error) {
	var m struct {
		CreateClientIntegration struct {
			Integration struct {
				Mrn string
			}
		} `graphql:"createClientIntegration(input: $input)"`
	}
	input := CreateClientIntegrationInput{
		SpaceMrn:             spaceMrn,
		Name:                 name,
		Type:                 clientIntegrationTypeAction1,
		ConfigurationOptions: action1Options(region, organizationID, &credentialMrn),
	}
	tflog.Trace(ctx, "CreateClientIntegrationInput", map[string]interface{}{"input": fmt.Sprintf("%+v", input)})
	if err := c.Mutate(ctx, &m, input, nil); err != nil {
		return "", err
	}
	return m.CreateClientIntegration.Integration.Mrn, nil
}

// UpdateAction1Integration updates an Action1 integration. A nil credentialMrn
// keeps the credential the integration has.
func (c *ExtendedGqlClient) UpdateAction1Integration(ctx context.Context, mrn, name, region, organizationID string, credentialMrn *string) error {
	var m struct {
		UpdateClientIntegrationConfiguration struct {
			Name string
		} `graphql:"updateClientIntegrationConfiguration(input: $input)"`
	}
	input := UpdateClientIntegrationConfigurationInput{
		Mrn:                  mrn,
		Name:                 name,
		Type:                 clientIntegrationTypeAction1,
		ConfigurationOptions: action1Options(region, organizationID, credentialMrn),
	}
	tflog.Trace(ctx, "UpdateClientIntegrationConfigurationInput", map[string]interface{}{"input": fmt.Sprintf("%+v", input)})
	return c.Mutate(ctx, &m, input, nil)
}

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

// CreateAction1Credential stores an Action1 API client as a typed credential in
// the given scope and returns its MRN.
func (c *ExtendedGqlClient) CreateAction1Credential(ctx context.Context, scopeMrn, name, clientID, clientSecret string) (string, error) {
	var m struct {
		CreateCredentialV2 struct {
			Credential struct {
				Mrn string
			}
		} `graphql:"createCredentialV2(input: $input)"`
	}
	input := CreateCredentialV2Input{
		ScopeMrn: scopeMrn,
		Name:     name,
		Secret: action1CredentialV2SecretInput{
			Action1: &Action1CredentialV2Input{ClientId: clientID, ClientSecret: clientSecret},
		},
	}
	tflog.Trace(ctx, "CreateCredentialV2", map[string]interface{}{"scope": scopeMrn, "name": name})
	if err := c.Mutate(ctx, &m, input, nil); err != nil {
		return "", err
	}
	return m.CreateCredentialV2.Credential.Mrn, nil
}

// RotateAction1Credential replaces the API client stored in an Action1 typed credential.
func (c *ExtendedGqlClient) RotateAction1Credential(ctx context.Context, mrn, clientID, clientSecret string) error {
	var m struct {
		RotateCredentialV2 struct {
			Credential struct {
				Mrn string
			}
		} `graphql:"rotateCredentialV2(input: $input)"`
	}
	input := RotateCredentialV2Input{
		Mrn: mrn,
		Secret: action1CredentialV2SecretInput{
			Action1: &Action1CredentialV2Input{ClientId: clientID, ClientSecret: clientSecret},
		},
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
