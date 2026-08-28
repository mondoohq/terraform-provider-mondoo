// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// credentialMrnDeprecationMessage is attached to every inline secret a typed
// credential replaces.
//
// It does not warn about replacement. An integration the platform has already
// moved onto a typed credential — by migration, or by minting one when it was
// created — adopts credential_mrn in place; only one that still stores its
// secret inline is replaced, and planCredentialBindingReplacement says so at
// plan time, against the integration in front of it rather than in advance.
const credentialMrnDeprecationMessage = "Use `credential_mrn` with a `mondoo_credential` resource instead. " +
	"This attribute will be removed in a future version."

// credentialBindingChange is what a plan does to credential_mrn.
type credentialBindingChange int

const (
	// credentialBindingUnchanged covers no change and a swap of one credential
	// MRN for another, which the server re-points in place.
	credentialBindingUnchanged credentialBindingChange = iota
	// credentialBindingDropped is the reference going away. The integration
	// stays credential-backed and the inline secret this update carries becomes
	// a rotation of the credential behind it.
	credentialBindingDropped
	// credentialBindingAdded is the reference appearing. The only change that
	// can need a replacement, and only when the integration is still inline.
	credentialBindingAdded
)

// classifyCredentialBinding reports what a plan does to credential_mrn.
//
// An unknown planned value is a reference to a credential being created in the
// same run. It is an addition, but only ever on a resource being created
// alongside it, which the caller has already returned on — so it classifies as
// unchanged rather than forcing a lookup against an integration that does not
// exist yet.
func classifyCredentialBinding(stateMrn, planMrn types.String) credentialBindingChange {
	if planMrn.IsUnknown() {
		return credentialBindingUnchanged
	}
	if stateMrn.IsNull() == planMrn.IsNull() {
		return credentialBindingUnchanged
	}
	if planMrn.IsNull() {
		return credentialBindingDropped
	}
	return credentialBindingAdded
}

// planCredentialBindingReplacement decides whether a change to credential_mrn
// needs the integration replaced, and is the whole of that decision — the
// attribute carries no RequiresReplace plan modifier.
//
// The server refuses exactly one move: adding a credential reference to an
// integration that still stores its secret inline, which it answers with
// "moving an existing integration onto typed credentials is a separate
// migration". Everything else it takes in place, and a plan modifier reading
// only prior state cannot tell the difference:
//
//   - already credential-backed, reference added: re-points the integration
//   - already credential-backed, reference removed: the inline secret becomes a
//     rotation of the credential behind it, and the integration stays V2
//   - already credential-backed, reference swapped: re-points
//   - still inline, no reference: the secret goes to the vault as it always has
//
// The second case is why removing credential_mrn is not a replacement despite
// looking like one: the server keeps the integration on its credential and
// rotates it, which is what lets a configuration that has always sent its
// secret inline keep working after the integration is migrated underneath it.
//
// Whether the integration is credential-backed is read from the API rather than
// from state, because state cannot know: a migration, or a credential minted
// when the integration was created, both happen server-side with nothing
// written back to Terraform.
//
// On a failed lookup this does NOT fall back to replacing. An unnecessary
// destroy of a live integration is worse than an apply that fails with the
// server's own message, so the server stays the authority.
func planCredentialBindingReplacement(
	ctx context.Context,
	client *ExtendedGqlClient,
	req resource.ModifyPlanRequest,
	resp *resource.ModifyPlanResponse,
) {
	// Create and destroy, where every attribute transitions by definition.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	credentialMrn := path.Root("credential_mrn")

	var stateMrn, planMrn types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, credentialMrn, &stateMrn)...)
	resp.Diagnostics.Append(req.Plan.GetAttribute(ctx, credentialMrn, &planMrn)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Only adding a reference can need a replacement; everything else the
	// server takes in place.
	if classifyCredentialBinding(stateMrn, planMrn) != credentialBindingAdded {
		return
	}

	var mrn types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("mrn"), &mrn)...)
	if resp.Diagnostics.HasError() || mrn.ValueString() == "" {
		return
	}

	integration, err := client.GetClientIntegration(ctx, mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddWarning(
			"Could not determine the integration's credential model",
			fmt.Sprintf(
				"Unable to read integration %q while planning: %s\n\n"+
					"The plan assumes this integration already authenticates with a typed credential, so it "+
					"proposes an in-place update. If it still stores its secret inline, the apply will fail "+
					"and report that moving it onto typed credentials is a separate migration.",
				mrn.ValueString(), err,
			),
		)
		return
	}

	if integration.TypedCredentialMrn(defaultCredentialPurpose).IsNull() {
		resp.RequiresReplace = append(resp.RequiresReplace, credentialMrn)
	}
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
				"Setting this on an integration that still stores its secret inline replaces the integration, "+
				"because moving it onto typed credentials is a separate migration; on one the platform has "+
				"already moved onto a credential it re-points in place, as does swapping it for another.",
			kind,
		),
		Optional: true,
		Validators: []validator.String{
			stringvalidator.ConflictsWith(conflicts...),
		},
	}
}
