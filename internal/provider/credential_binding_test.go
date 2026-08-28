// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// schemaAttributes fetches a resource's top-level attributes for assertions.
func schemaAttributes(t *testing.T, r resource.Resource) map[string]schema.Attribute {
	t.Helper()

	resp := &resource.SchemaResponse{}
	r.Schema(context.Background(), resource.SchemaRequest{}, resp)
	if resp.Diagnostics.HasError() {
		t.Fatalf("schema: %v", resp.Diagnostics)
	}
	return resp.Schema.Attributes
}

// The server refuses exactly one move — adding a reference to an integration
// that still stores its secret inline. These pin the classification that
// decides whether the API is even consulted; only credentialBindingAdded can
// lead to a replacement.
func TestClassifyCredentialBinding(t *testing.T) {
	mrnA := types.StringValue("//credential/1")
	mrnB := types.StringValue("//credential/2")

	tests := []struct {
		name      string
		state     types.String
		plan      types.String
		expected  credentialBindingChange
		reasoning string
	}{
		{
			name: "adding a reference", state: types.StringNull(), plan: mrnA,
			expected:  credentialBindingAdded,
			reasoning: "the only change the server can refuse, and only when the integration is still inline",
		},
		{
			name: "dropping a reference", state: mrnA, plan: types.StringNull(),
			expected:  credentialBindingDropped,
			reasoning: "the integration stays credential-backed and the inline secret rotates the credential behind it",
		},
		{
			name: "swapping one credential for another", state: mrnA, plan: mrnB,
			expected:  credentialBindingUnchanged,
			reasoning: "a re-point, which the server takes in place",
		},
		{
			name: "no reference either side", state: types.StringNull(), plan: types.StringNull(),
			expected:  credentialBindingUnchanged,
			reasoning: "an inline integration staying inline",
		},
		{
			name: "same reference either side", state: mrnA, plan: mrnA,
			expected:  credentialBindingUnchanged,
			reasoning: "no change at all",
		},
		{
			name: "unknown until apply", state: types.StringNull(), plan: types.StringUnknown(),
			expected:  credentialBindingUnchanged,
			reasoning: "a credential created in the same run, on an integration created alongside it",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyCredentialBinding(tt.state, tt.plan); got != tt.expected {
				t.Errorf("got %v, want %v — %s", got, tt.expected, tt.reasoning)
			}
		})
	}
}

// A migrated or auto-minted integration reports the credential governing it, so
// adopting credential_mrn re-points it rather than destroying it. An integration
// still holding its secret inline reports none.
func TestTypedCredentialMrnDecidesReplacement(t *testing.T) {
	backed := Integration{Credentials: []IntegrationCredential{
		{Purpose: defaultCredentialPurpose, Mrn: "//credential/1"},
	}}
	if backed.TypedCredentialMrn(defaultCredentialPurpose).IsNull() {
		t.Error("a credential-backed integration must not be replaced to adopt credential_mrn")
	}

	inline := Integration{}
	if !inline.TypedCredentialMrn(defaultCredentialPurpose).IsNull() {
		t.Error("an inline integration must be replaced: the server refuses the move in place")
	}
}

// credential_mrn must carry no RequiresReplace plan modifier: the decision needs
// the API, so it lives in ModifyPlan. A modifier here would replace integrations
// the server would have re-pointed in place.
func TestCredentialMrnHasNoStaticReplaceModifier(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    resource.Resource
	}{
		{"slack", NewIntegrationSlackResource()},
		{"github", NewIntegrationGithubResource()},
		{"aws", NewIntegrationAwsResource()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			attr, ok := schemaAttributes(t, tc.r)["credential_mrn"].(schema.StringAttribute)
			if !ok {
				t.Fatal("credential_mrn is missing or not a StringAttribute")
			}
			if len(attr.PlanModifiers) != 0 {
				t.Errorf("credential_mrn has %d plan modifiers; the replacement decision belongs in ModifyPlan, "+
					"which can tell a migrated integration from an inline one", len(attr.PlanModifiers))
			}
		})
	}
}

// Every integration that can bind a credential must implement ModifyPlan, or it
// silently loses the replacement decision entirely.
func TestCredentialBoundResourcesImplementModifyPlan(t *testing.T) {
	for _, tc := range []struct {
		name string
		r    resource.Resource
	}{
		{"slack", NewIntegrationSlackResource()},
		{"github", NewIntegrationGithubResource()},
		{"aws", NewIntegrationAwsResource()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := tc.r.(resource.ResourceWithModifyPlan); !ok {
				t.Error("does not implement ResourceWithModifyPlan")
			}
		})
	}
}

func TestSlackInlineSecretIsDeprecated(t *testing.T) {
	attrs := schemaAttributes(t, NewIntegrationSlackResource())

	token, ok := attrs["slack_token"]
	if !ok {
		t.Fatal("slack_token is missing")
	}
	if token.GetDeprecationMessage() == "" {
		t.Error("slack_token must be deprecated in favour of credential_mrn")
	}
	if token.IsRequired() {
		t.Error("slack_token must be Optional so ExactlyOneOf can reference it")
	}
	if _, ok := attrs["credential_mrn"]; !ok {
		t.Error("credential_mrn is missing")
	}
}

func TestGithubInlineSecretIsDeprecated(t *testing.T) {
	attrs := schemaAttributes(t, NewIntegrationGithubResource())

	creds, ok := attrs["credentials"]
	if !ok {
		t.Fatal("credentials is missing")
	}
	if creds.GetDeprecationMessage() == "" {
		t.Error("credentials must be deprecated in favour of credential_mrn")
	}
	if creds.IsRequired() {
		t.Error("credentials must be Optional so ExactlyOneOf can reference it")
	}
	if _, ok := attrs["credential_mrn"]; !ok {
		t.Error("credential_mrn is missing")
	}
}

// AWS deprecates credentials.key only. role and wif have no credential
// equivalent, so neither they nor the block itself may carry a deprecation —
// that would tell those users to migrate to something that cannot hold their
// authentication mode.
func TestAwsDeprecatesOnlyTheKeyArm(t *testing.T) {
	attrs := schemaAttributes(t, NewIntegrationAwsResource())

	creds, ok := attrs["credentials"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatalf("credentials is %T, want schema.SingleNestedAttribute", attrs["credentials"])
	}
	if creds.GetDeprecationMessage() != "" {
		t.Error("the credentials block must NOT be deprecated: role and wif remain the recommendation")
	}
	if creds.IsRequired() {
		t.Error("credentials must be Optional so ExactlyOneOf can reference credential_mrn")
	}

	if msg := creds.Attributes["key"].GetDeprecationMessage(); msg == "" {
		t.Error("credentials.key must be deprecated in favour of credential_mrn")
	}
	for _, arm := range []string{"role", "wif"} {
		if msg := creds.Attributes[arm].GetDeprecationMessage(); msg != "" {
			t.Errorf("credentials.%s must NOT be deprecated; credential_mrn cannot express it (got %q)", arm, msg)
		}
	}

	if _, ok := attrs["credential_mrn"]; !ok {
		t.Error("credential_mrn is missing")
	}
}
