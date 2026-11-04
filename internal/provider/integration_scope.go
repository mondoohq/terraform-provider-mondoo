// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// orgScopedIntegrationDescription is the sentence every integration resource that
// can be created in an organization adds to its description.
const orgScopedIntegrationDescription = "Create it in a Mondoo organization (`org_id`), whose asset routing rules place the assets into spaces, or in a space (`space_id`)."

// integrationOrgIDAttribute is the `org_id` attribute of an integration that can
// be created in an organization.
func integrationOrgIDAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: "Mondoo organization identifier. Set this to create the integration in the organization. Conflicts with `space_id`.",
		Optional:            true,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.RequiresReplace(),
		},
		Validators: []validator.String{
			stringvalidator.ConflictsWith(path.MatchRoot("space_id")),
		},
	}
}

// integrationSpaceIDAttribute is the `space_id` attribute next to `org_id`.
// Its plan modifiers stay what these resources always had: adding org_id did not
// change how an existing space_id plans. Moving between the organization and a
// space still replaces the integration, through org_id's RequiresReplace, which
// fires when org_id is set or cleared.
func integrationSpaceIDAttribute() schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: "Mondoo space identifier. If neither `space_id` nor `org_id` is set, the provider space is used.",
		Optional:            true,
		Computed:            true,
		PlanModifiers: []planmodifier.String{
			stringplanmodifier.UseStateForUnknown(),
		},
	}
}

// integrationScopeMrn returns the scope to create an integration in, and the
// space_id to store: the organization when org_id is set (space_id null), else
// the configured space or the provider space.
func integrationScopeMrn(client *ExtendedGqlClient, orgID, spaceID types.String) (string, types.String, error) {
	if id := orgID.ValueString(); id != "" {
		return orgPrefix + id, types.StringNull(), nil
	}
	space, err := client.ComputeSpace(spaceID)
	if err != nil {
		return "", types.StringNull(), err
	}
	return space.MRN(), types.StringValue(space.ID()), nil
}

// integrationScope maps an integration read from the API to org_id and space_id:
// exactly one is set, depending on whether the integration lives in an
// organization or in a space.
func integrationScope(integration Integration) (orgID, spaceID types.String) {
	if id, ok := strings.CutPrefix(integration.ScopeMRN(), orgPrefix); ok {
		return types.StringValue(id), types.StringNull()
	}
	return types.StringNull(), types.StringValue(integration.SpaceID())
}
