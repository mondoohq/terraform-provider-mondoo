// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/base64"
	mondoov1 "go.mondoo.com/mondoo-go"
)

type ExtendedGqlClient struct {
	*mondoov1.Client
}

// newDataUrl generates a https://tools.ietf.org/html/rfc2397 data url for a given content.
func newDataUrl(content []byte) string {
	return "data:application/x-yaml;base64," + base64.StdEncoding.EncodeToString(content)
}

type setCustomPolicyRequest struct {
	SetCustomPolicyPayload struct {
		PolicyMrns []mondoov1.String
	} `graphql:"setCustomPolicy(input: $input)"`
}

func (c *ExtendedGqlClient) SetCustomPolicy(ctx context.Context, scopeMrn string, overwriteVal *bool, policyBundleData []byte) (setCustomPolicyRequest, error) {
	var overwrite *mondoov1.Boolean
	if overwriteVal != nil {
		overwrite = mondoov1.NewBooleanPtr(mondoov1.Boolean(*overwriteVal))
	}

	setCustomPolicyInput := mondoov1.SetCustomPolicyInput{
		SpaceMrn:  mondoov1.String(scopeMrn),
		Overwrite: overwrite,
		Dataurl:   mondoov1.String(newDataUrl(policyBundleData)),
	}

	var setCustomPolicy setCustomPolicyRequest

	err := c.Mutate(ctx, &setCustomPolicy, []mondoov1.SetCustomPolicyInput{setCustomPolicyInput}, nil)
	return setCustomPolicy, err
}

func (c *ExtendedGqlClient) AssignPolicy(ctx context.Context, spaceMrn string, policyMrns []string) error {
	var list *[]mondoov1.String

	entries := []mondoov1.String{}
	for i := range policyMrns {
		entries = append(entries, mondoov1.String(policyMrns[i]))
	}

	if len(entries) > 0 {
		list = &entries
	}
	action := mondoov1.PolicyActionActive

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
