// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

type ExtendedGqlClient struct {
	*mondoov1.Client
}

// newDataUrl generates a https://tools.ietf.org/html/rfc2397 data url for a given content.
func newDataUrl(content []byte) string {
	return "data:application/x-yaml;base64," + base64.StdEncoding.EncodeToString(content)
}

type createSpacePayload struct {
	Id   mondoov1.ID
	Mrn  mondoov1.String
	Name mondoov1.String
}

func (c *ExtendedGqlClient) CreateSpace(ctx context.Context, orgID string, id string, name string) (createSpacePayload, error) {
	var createMutation struct {
		CreateSpace createSpacePayload `graphql:"createSpace(input: $input)"`
	}

	var spaceID *mondoov1.String
	if id != "" {
		spaceID = mondoov1.NewStringPtr(mondoov1.String(id))
	}

	createInput := mondoov1.CreateSpaceInput{
		Name:   mondoov1.String(name),
		ID:     spaceID,
		OrgMrn: mondoov1.String(orgPrefix + orgID),
	}

	tflog.Trace(ctx, "CreateSpaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", createInput),
	})

	err := c.Mutate(ctx, &createMutation, createInput, nil)
	return createMutation.CreateSpace, err
}

func (c *ExtendedGqlClient) UpdateSpace(ctx context.Context, spaceID string, name string) error {
	var updateMutation struct {
		UpdateSpace struct {
			Space struct {
				Mrn  mondoov1.String
				Name mondoov1.String
			}
		} `graphql:"updateSpace(input: $input)"`
	}
	updateInput := mondoov1.UpdateSpaceInput{
		Mrn:  mondoov1.String(spacePrefix + spaceID),
		Name: mondoov1.String(name),
	}
	tflog.Trace(ctx, "UpdateSpaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", updateInput),
	})
	return c.Mutate(ctx, &updateMutation, updateInput, nil)
}

func (c *ExtendedGqlClient) DeleteSpace(ctx context.Context, spaceID string) error {
	var deleteMutation struct {
		DeleteSpace mondoov1.String `graphql:"deleteSpace(spaceMrn: $spaceMrn)"`
	}
	variables := map[string]interface{}{
		"spaceMrn": mondoov1.ID(spacePrefix + spaceID),
	}

	tflog.Trace(ctx, "DeleteSpaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", variables),
	})

	return c.Mutate(ctx, &deleteMutation, nil, variables)
}

type spacePayload struct {
	Id           string
	Mrn          string
	Name         string
	Organization struct {
		Id string
	}
}

func (c *ExtendedGqlClient) GetSpace(ctx context.Context, mrn string) (spacePayload, error) {
	var q struct {
		Space spacePayload `graphql:"space(mrn: $mrn)"`
	}
	variables := map[string]interface{}{
		"mrn": mondoov1.String(mrn),
	}

	err := c.Query(ctx, &q, variables)
	if err != nil {
		return spacePayload{}, err
	}

	return q.Space, nil
}

type orgPayload struct {
	Id   string
	Mrn  string
	Name string
}

func (c *ExtendedGqlClient) GetOrganization(ctx context.Context, mrn string) (orgPayload, error) {
	var q struct {
		Organization orgPayload `graphql:"organization(mrn: $mrn)"`
	}
	variables := map[string]interface{}{
		"mrn": mondoov1.String(mrn),
	}

	err := c.Query(ctx, &q, variables)
	if err != nil {
		return orgPayload{}, err
	}

	return q.Organization, nil
}

type setCustomPolicyPayload struct {
	PolicyMrns []mondoov1.String
}

func (c *ExtendedGqlClient) SetCustomPolicy(ctx context.Context, scopeMrn string, overwriteVal *bool, policyBundleData []byte) (setCustomPolicyPayload, error) {
	var overwrite *mondoov1.Boolean
	if overwriteVal != nil {
		overwrite = mondoov1.NewBooleanPtr(mondoov1.Boolean(*overwriteVal))
	}

	setCustomPolicyInput := mondoov1.SetCustomPolicyInput{
		SpaceMrn:  mondoov1.String(scopeMrn),
		Overwrite: overwrite,
		Dataurl:   mondoov1.String(newDataUrl(policyBundleData)),
	}

	var setCustomPolicy struct {
		SetCustomPolicyPayload setCustomPolicyPayload `graphql:"setCustomPolicy(input: $input)"`
	}

	err := c.Mutate(ctx, &setCustomPolicy, []mondoov1.SetCustomPolicyInput{setCustomPolicyInput}, nil)
	return setCustomPolicy.SetCustomPolicyPayload, err
}

func (c *ExtendedGqlClient) AssignPolicy(ctx context.Context, spaceMrn string, action mondoov1.PolicyAction, policyMrns []string) error {
	var list *[]mondoov1.String

	entries := []mondoov1.String{}
	for i := range policyMrns {
		entries = append(entries, mondoov1.String(policyMrns[i]))
	}

	if len(entries) > 0 {
		list = &entries
	}

	policyAssignmentInput := mondoov1.PolicyAssignmentInput{
		AssetMrn:   mondoov1.String(spaceMrn),
		PolicyMrns: list,
		Action:     &action,
	}

	var policyAssignment struct {
		AssignPolicy bool `graphql:"assignPolicy(input: $input)"`
	}

	return c.Mutate(ctx, &policyAssignment, policyAssignmentInput, nil)
}

func (c *ExtendedGqlClient) UnassignPolicy(ctx context.Context, spaceMrn string, policyMrns []string) error {
	var list *[]mondoov1.String

	entries := []mondoov1.String{}
	for i := range policyMrns {
		entries = append(entries, mondoov1.String(policyMrns[i]))
	}

	if len(entries) > 0 {
		list = &entries
	}

	policyAssignmentInput := mondoov1.PolicyAssignmentInput{
		AssetMrn:   mondoov1.String(spaceMrn),
		PolicyMrns: list,
	}

	var policyAssignment struct {
		AssignPolicy bool `graphql:"unassignPolicy(input: $input)"`
	}

	return c.Mutate(ctx, &policyAssignment, policyAssignmentInput, nil)
}

type SetCustomPolicyPayload struct {
	QueryPackMrns []mondoov1.String
}

func (c *ExtendedGqlClient) SetCustomQueryPack(ctx context.Context, scopeMrn string, overwriteVal *bool, policyBundleData []byte) (SetCustomPolicyPayload, error) {
	var overwrite *mondoov1.Boolean
	if overwriteVal != nil {
		overwrite = mondoov1.NewBooleanPtr(mondoov1.Boolean(*overwriteVal))
	}

	setCustomPolicyInput := mondoov1.SetCustomQueryPackInput{
		SpaceMrn:  mondoov1.String(scopeMrn),
		Overwrite: overwrite,
		Dataurl:   mondoov1.String(newDataUrl(policyBundleData)),
	}

	var setCustomQueryPackPayload struct {
		SetCustomPolicyPayload SetCustomPolicyPayload `graphql:"setCustomQueryPack(input: $input)"`
	}

	err := c.Mutate(ctx, &setCustomQueryPackPayload, []mondoov1.SetCustomQueryPackInput{setCustomPolicyInput}, nil)
	return setCustomQueryPackPayload.SetCustomPolicyPayload, err
}

func (c *ExtendedGqlClient) DeletePolicy(ctx context.Context, policyMrn string) error {
	deleteCustomPolicyInput := mondoov1.DeleteCustomPolicyInput{
		PolicyMrn: mondoov1.String(policyMrn),
	}

	var deleteCustomPolicy struct {
		DeleteCustomPolicyPayload struct {
			PolicyMrn mondoov1.String
		} `graphql:"deleteCustomPolicy(input: $input)"`
	}

	return c.Mutate(ctx, &deleteCustomPolicy, deleteCustomPolicyInput, nil)
}

type CreateClientIntegrationPayload struct {
	Mrn  mondoov1.String
	Name mondoov1.String
}

func (c *ExtendedGqlClient) CreateIntegration(ctx context.Context, spaceMrn, name string, typ mondoov1.ClientIntegrationType, opts mondoov1.ClientIntegrationConfigurationInput) (*CreateClientIntegrationPayload, error) {
	var createMutation struct {
		CreateClientIntegration struct {
			Integration CreateClientIntegrationPayload
		} `graphql:"createClientIntegration(input: $input)"`
	}

	createInput := mondoov1.CreateClientIntegrationInput{
		SpaceMrn:             mondoov1.String(spaceMrn),
		Name:                 mondoov1.String(name),
		Type:                 typ,
		LongLivedToken:       false,
		ConfigurationOptions: opts,
	}

	tflog.Trace(ctx, "CreateSpaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", createInput),
	})

	err := c.Mutate(ctx, &createMutation, createInput, nil)
	if err != nil {
		return nil, err
	}
	return &createMutation.CreateClientIntegration.Integration, nil
}

type UpdateIntegrationPayload struct {
	Name mondoov1.String
}

func (c *ExtendedGqlClient) UpdateIntegration(ctx context.Context, mrn, name string, typ mondoov1.ClientIntegrationType, opts mondoov1.ClientIntegrationConfigurationInput) (*UpdateIntegrationPayload, error) {
	var updateMutation struct {
		UpdateIntegrationPayload `graphql:"updateClientIntegrationConfiguration(input: $input)"`
	}

	updateInput := mondoov1.UpdateClientIntegrationConfigurationInput{
		Mrn:                  mondoov1.String(mrn),
		Name:                 mondoov1.NewStringPtr(mondoov1.String(name)),
		Type:                 typ,
		ConfigurationOptions: opts,
	}
	tflog.Trace(ctx, "UpdateClientIntegrationNameInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", updateInput),
	})
	err := c.Mutate(ctx, &updateMutation, updateInput, nil)
	if err != nil {
		return nil, err
	}
	return &updateMutation.UpdateIntegrationPayload, nil
}

type DeleteIntegrationPayload struct {
	Mrn mondoov1.String
}

func (c *ExtendedGqlClient) DeleteIntegration(ctx context.Context, mrn string) (*DeleteIntegrationPayload, error) {
	var deleteMutation struct {
		DeleteClientIntegration DeleteIntegrationPayload `graphql:"deleteClientIntegration(input: $input)"`
	}
	deleteInput := mondoov1.DeleteClientIntegrationInput{
		Mrn: mondoov1.String(mrn),
	}
	tflog.Trace(ctx, "DeleteClientIntegration", map[string]interface{}{
		"input": fmt.Sprintf("%+v", deleteInput),
	})
	err := c.Mutate(ctx, &deleteMutation, deleteInput, nil)
	if err != nil {
		return nil, err
	}
	return &deleteMutation.DeleteClientIntegration, nil
}

type triggerActionPayload struct {
	Mrn string
}

func (c *ExtendedGqlClient) TriggerAction(ctx context.Context, integrationMrn string, action mondoov1.ActionType) (triggerActionPayload, error) {

	var q struct {
		TriggerAction triggerActionPayload `graphql:"triggerAction(input: { mrn: $mrn, type: $type })"`
	}
	variables := map[string]interface{}{
		"mrn":  mondoov1.String(integrationMrn),
		"type": action,
	}

	err := c.Query(ctx, &q, variables)
	if err != nil {
		return triggerActionPayload{}, err
	}

	return q.TriggerAction, nil
}

func (c *ExtendedGqlClient) SetScimGroupMapping(ctx context.Context, orgMrn string, group string, mappings []mondoov1.ScimGroupMapping) error {
	var setScimGroupMappingMutation struct {
		SetScimGroupMapping struct {
			Group mondoov1.String
		} `graphql:"setScimGroupMapping(input: $input)"`
	}

	setScimGroupMappingInput := mondoov1.SetScimGroupMappingInput{
		OrgMrn:   mondoov1.String(orgMrn),
		Group:    mondoov1.String(group),
		Mappings: mappings,
	}

	return c.Mutate(ctx, &setScimGroupMappingMutation, setScimGroupMappingInput, nil)
}
