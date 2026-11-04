// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testOptOutOrgMrn = "//captain.api.mondoo.app/organizations/my-org-123456"

func TestOrganizationPlatformPoliciesOptOutSchemaIsValid(t *testing.T) {
	ctx := context.Background()
	var resp resource.SchemaResponse
	NewOrganizationPlatformPoliciesOptOutResource().Schema(ctx, resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	diags := resp.Schema.ValidateImplementation(ctx)
	require.False(t, diags.HasError(), "%v", diags)
}

// The documents must match the API's GraphQL schema. mondoo-go does not carry
// this mutation and field yet, so they are pinned here.

func TestSetOrganizationPlatformPoliciesOptOutRequest(t *testing.T) {
	for _, optedOut := range []bool{true, false} {
		c, got := newGraphqlTestClient(t, `{"data":{"setOrganizationPlatformPoliciesOptOut":true}}`)

		require.NoError(t, c.SetOrganizationPlatformPoliciesOptOut(context.Background(), testOptOutOrgMrn, optedOut))

		assert.Equal(t,
			`mutation($input:SetOrganizationPlatformPoliciesOptOutInput!){setOrganizationPlatformPoliciesOptOut(input: $input)}`,
			got.Query)
		want := `{"organizationMrn":"` + testOptOutOrgMrn + `","optedOut":false}`
		if optedOut {
			want = `{"organizationMrn":"` + testOptOutOrgMrn + `","optedOut":true}`
		}
		assert.JSONEq(t, want, string(got.Variables["input"]))
	}
}

func TestGetOrganizationPlatformPoliciesOptedOutRequest(t *testing.T) {
	c, got := newGraphqlTestClient(t, `{"data":{"organization":{"settings":{"platformPoliciesOptedOut":true}}}}`)

	optedOut, err := c.GetOrganizationPlatformPoliciesOptedOut(context.Background(), testOptOutOrgMrn)
	require.NoError(t, err)
	assert.True(t, optedOut)
	assert.Equal(t,
		`query($mrn:String!){organization(mrn: $mrn){settings{platformPoliciesOptedOut}}}`,
		got.Query)
	assert.JSONEq(t, `"`+testOptOutOrgMrn+`"`, string(got.Variables["mrn"]))
}

// A failed read must not look like "not opted out": the resource would drop from
// state and the next apply would act on an organization it knows nothing about.
func TestGetOrganizationPlatformPoliciesOptedOutErrorIsReturned(t *testing.T) {
	c, _ := newGraphqlTestClient(t, `{"errors":[{"message":"rpc error: code = PermissionDenied desc = no valid permissions to access the resource"}],"data":null}`)

	_, err := c.GetOrganizationPlatformPoliciesOptedOut(context.Background(), testOptOutOrgMrn)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PermissionDenied")
	assert.False(t, isOrganizationNotFoundError(err))
}

// A deleted organization must drop from state, or every later plan fails.
func TestIsOrganizationNotFoundError(t *testing.T) {
	assert.True(t, isOrganizationNotFoundError(errors.New("client: organization not found")))
	assert.True(t, isOrganizationNotFoundError(errors.New("rpc error: code = NotFound desc = no such resource")))
	assert.False(t, isOrganizationNotFoundError(errors.New("rpc error: code = PermissionDenied desc = no valid permissions to access the resource")))
	assert.False(t, isOrganizationNotFoundError(nil))
}

func TestOrganizationMrnRegexp(t *testing.T) {
	assert.True(t, organizationMrnRegexp.MatchString(testOptOutOrgMrn))
	for _, mrn := range []string{
		"",
		"my-org-123456",
		orgPrefix,
		orgPrefix + "my-org-123456/spaces/my-space",
		orgPrefix + "my org",
		"//captain.api.mondoo.app/spaces/my-space-123456",
	} {
		assert.False(t, organizationMrnRegexp.MatchString(mrn), mrn)
	}
}
