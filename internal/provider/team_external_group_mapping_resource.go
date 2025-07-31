// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TeamExternalGroupMappingResource{}
var _ resource.ResourceWithImportState = &TeamExternalGroupMappingResource{}

func NewTeamExternalGroupMappingResource() resource.Resource {
	return &TeamExternalGroupMappingResource{}
}

// TeamExternalGroupMappingResource defines the resource implementation.
type TeamExternalGroupMappingResource struct {
	client *ExtendedGqlClient
}

// TeamExternalGroupMappingResourceModel describes the resource data model.
type TeamExternalGroupMappingResourceModel struct {
	Mrn        types.String `tfsdk:"mrn"`
	TeamMrn    types.String `tfsdk:"team_mrn"`
	ExternalId types.String `tfsdk:"external_id"`
	CreatedAt  types.String `tfsdk:"created_at"`
}

func (r *TeamExternalGroupMappingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_external_group_mapping"
}

func (r *TeamExternalGroupMappingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `This resource manages external group mappings for Mondoo Teams. External group mappings link OIDC group claims to teams, enabling automatic team membership based on identity provider group membership.`,

		Attributes: map[string]schema.Attribute{
			"mrn": schema.StringAttribute{
				MarkdownDescription: "Mondoo Resource Name (MRN) of the team external group mapping.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"team_mrn": schema.StringAttribute{
				MarkdownDescription: "MRN of the team to map the external group to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"external_id": schema.StringAttribute{
				MarkdownDescription: "External group ID from the OIDC provider (e.g., group name or UUID).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp when the team external group mapping was created.",
				Computed:            true,
			},
		},
	}
}

func (r *TeamExternalGroupMappingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ExtendedGqlClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *TeamExternalGroupMappingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TeamExternalGroupMappingResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create the team external group mapping
	input := AddTeamExternalGroupMappingInput{
		TeamMrn:    mondoov1.String(data.TeamMrn.ValueString()),
		ExternalId: mondoov1.String(data.ExternalId.ValueString()),
	}

	payload, err := r.client.AddTeamExternalGroupMapping(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create team external group mapping, got error: %s", err))
		return
	}

	// Map response back to schema
	data.Mrn = types.StringValue(string(payload.Mrn))
	data.TeamMrn = types.StringValue(string(payload.Team.Mrn))
	data.ExternalId = types.StringValue(string(payload.ExternalId))
	data.CreatedAt = types.StringValue(string(payload.CreatedAt))

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamExternalGroupMappingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TeamExternalGroupMappingResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Get the team external group mapping from the API
	payload, err := r.client.GetTeamExternalGroupMapping(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read team external group mapping, got error: %s", err))
		return
	}

	// Map response back to schema
	data.Mrn = types.StringValue(string(payload.Mrn))
	data.TeamMrn = types.StringValue(string(payload.Team.Mrn))
	data.ExternalId = types.StringValue(string(payload.ExternalId))
	data.CreatedAt = types.StringValue(string(payload.CreatedAt))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamExternalGroupMappingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Team external group mappings are immutable - both team_mrn and external_id require replacement
	// This method should never be called due to RequiresReplace plan modifiers
	resp.Diagnostics.AddError(
		"Unexpected Update Call",
		"Team external group mappings cannot be updated. Both team_mrn and external_id changes require replacement.",
	)
}

func (r *TeamExternalGroupMappingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TeamExternalGroupMappingResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the team external group mapping
	err := r.client.RemoveTeamExternalGroupMapping(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete team external group mapping, got error: %s", err))
		return
	}
}

func (r *TeamExternalGroupMappingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("mrn"), req, resp)
}