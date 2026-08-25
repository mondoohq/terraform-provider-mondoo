// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// credentialMrnDeprecationMessage is attached to every inline secret a typed
// credential replaces. The third sentence matters: because of
// requiresReplaceOnTransition, adding credential_mrn to a live integration is a
// destroy-and-create rather than an edit.
const credentialMrnDeprecationMessage = "Use `credential_mrn` with a `mondoo_credential` resource instead. " +
	"This attribute will be removed in a future version. Note that switching an existing integration to " +
	"`credential_mrn` replaces the integration."

// requiresReplaceOnTransition replaces the resource when the attribute appears
// or disappears, but not when its value changes.
//
// An integration cannot be moved between the inline-secret and credential-
// backed models after creation, so null-to-known and known-to-null are both
// replacements. Swapping one credential MRN for another is supported in place.
// Without this, removing credential_mrn from a configuration would plan an
// in-place update, fail at apply and re-propose the same update forever —
// omitting credentialMrn server-side keeps the current credential rather than
// clearing it.
//
// The guards skip create and destroy, where every attribute transitions by
// definition.
func requiresReplaceOnTransition() planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		func(ctx context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
				return
			}
			resp.RequiresReplace = req.StateValue.IsNull() != req.PlanValue.IsNull()
		},
		"Replaces the integration when it starts or stops using a typed credential.",
		"Replaces the integration when it starts or stops using a typed credential.",
	)
}

// credentialMrnAttribute builds the credential_mrn attribute shared by every
// integration that can authenticate through a typed credential. kind names the
// credential kind the integration accepts; conflicts lists the inline-secret
// paths it is mutually exclusive with.
func credentialMrnAttribute(kind string, conflicts ...path.Expression) schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: fmt.Sprintf(
			"MRN of an existing `mondoo_credential` to authenticate with, instead of supplying a secret inline. "+
				"Must be a `%s` credential owned by this integration's own scope — ownership is matched exactly, "+
				"so a space-level integration cannot use a credential owned by its organization, or the reverse. "+
				"An integration cannot be moved between the inline-secret and credential-backed models after "+
				"creation, so setting or removing this attribute replaces the integration; re-pointing it at a "+
				"different credential happens in place.",
			kind,
		),
		Optional: true,
		PlanModifiers: []planmodifier.String{
			requiresReplaceOnTransition(),
		},
		Validators: []validator.String{
			stringvalidator.ConflictsWith(conflicts...),
		},
	}
}
