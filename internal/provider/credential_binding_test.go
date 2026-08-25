// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
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

// nonNullRaw is any non-null object value; the modifier only tests IsNull() on
// the raw state and plan to detect create and destroy.
func nonNullRaw() tftypes.Value {
	objType := tftypes.Object{AttributeTypes: map[string]tftypes.Type{"credential_mrn": tftypes.String}}
	return tftypes.NewValue(objType, map[string]tftypes.Value{
		"credential_mrn": tftypes.NewValue(tftypes.String, "placeholder"),
	})
}

// runTransitionModifier drives the modifier and reports whether it asked for a
// replacement.
func runTransitionModifier(t *testing.T, rawState, rawPlan tftypes.Value, stateValue, planValue types.String) bool {
	t.Helper()

	req := planmodifier.StringRequest{
		State:      tfsdk.State{Raw: rawState},
		Plan:       tfsdk.Plan{Raw: rawPlan},
		StateValue: stateValue,
		PlanValue:  planValue,
	}
	resp := &planmodifier.StringResponse{PlanValue: planValue}

	requiresReplaceOnTransition().PlanModifyString(context.Background(), req, resp)

	return resp.RequiresReplace
}

func TestRequiresReplaceOnTransitionNullToKnown(t *testing.T) {
	got := runTransitionModifier(t, nonNullRaw(), nonNullRaw(), types.StringNull(), types.StringValue("//credential/1"))
	if !got {
		t.Error("null -> known must require replacement: an integration cannot move to the credential model in place")
	}
}

func TestRequiresReplaceOnTransitionKnownToNull(t *testing.T) {
	got := runTransitionModifier(t, nonNullRaw(), nonNullRaw(), types.StringValue("//credential/1"), types.StringNull())
	if !got {
		t.Error("known -> null must require replacement: omitting credentialMrn keeps the current credential server-side")
	}
}

func TestRequiresReplaceOnTransitionKnownToKnown(t *testing.T) {
	got := runTransitionModifier(t, nonNullRaw(), nonNullRaw(), types.StringValue("//credential/1"), types.StringValue("//credential/2"))
	if got {
		t.Error("swapping one credential MRN for another is supported in place and must not replace")
	}
}

func TestRequiresReplaceOnTransitionSkipsCreate(t *testing.T) {
	got := runTransitionModifier(t, tftypes.Value{}, nonNullRaw(), types.StringNull(), types.StringValue("//credential/1"))
	if got {
		t.Error("create must not be treated as a transition")
	}
}

func TestRequiresReplaceOnTransitionSkipsDestroy(t *testing.T) {
	got := runTransitionModifier(t, nonNullRaw(), tftypes.Value{}, types.StringValue("//credential/1"), types.StringNull())
	if got {
		t.Error("destroy must not be treated as a transition")
	}
}
