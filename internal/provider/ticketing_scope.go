// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

// Ticketing integrations at organization scope (planning#1632, server ADR-345):
// an organization's ticketing integration is shared configuration every space
// of the organization files its cases' tickets through. The server refuses
// auto-create on it, so the resources refuse it at plan time too. The scope
// itself (org_id, space_id) uses the shared helpers in integration_scope.go.

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// orgTicketingIntegrationDescription is the sentence every ticketing integration
// resource that can be created in an organization adds to its description.
const orgTicketingIntegrationDescription = "Create it in a Mondoo organization (`org_id`), " +
	"whose spaces can all file tickets through it, or in a space (`space_id`). " +
	"An organization integration cannot auto-create tickets."

// orgAutoCreateValidator refuses auto-create on an organization ticketing
// integration, which the server refuses too.
type orgAutoCreateValidator struct {
	autoCreateAttr string
}

var _ resource.ConfigValidator = orgAutoCreateValidator{}

func newOrgAutoCreateValidator(autoCreateAttr string) resource.ConfigValidator {
	return orgAutoCreateValidator{autoCreateAttr: autoCreateAttr}
}

func (v orgAutoCreateValidator) Description(_ context.Context) string {
	return fmt.Sprintf("%s cannot be enabled when org_id is set", v.autoCreateAttr)
}

func (v orgAutoCreateValidator) MarkdownDescription(ctx context.Context) string {
	return v.Description(ctx)
}

func (v orgAutoCreateValidator) ValidateResource(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var orgID types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("org_id"), &orgID)...)
	var autoCreate types.Bool
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(v.autoCreateAttr), &autoCreate)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Unknown values are checked again at apply, once known.
	if orgID.IsNull() || orgID.IsUnknown() || orgID.ValueString() == "" || !autoCreate.ValueBool() {
		return
	}
	resp.Diagnostics.AddAttributeError(
		path.Root(v.autoCreateAttr),
		"Invalid Configuration",
		fmt.Sprintf("`%s` is not available for an organization ticketing integration (`org_id`): "+
			"the organization's spaces file tickets through it per case. Remove `%s` or set `space_id` instead.",
			v.autoCreateAttr, v.autoCreateAttr),
	)
}
