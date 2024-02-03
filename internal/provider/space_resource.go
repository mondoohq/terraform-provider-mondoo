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
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

const (
	orgPrefix   = "//captain.api.mondoo.app/organizations/"
	spacePrefix = "//captain.api.mondoo.app/spaces/"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &SpaceResource{}
var _ resource.ResourceWithImportState = &SpaceResource{}

func NewSpaceResource() resource.Resource {
	return &SpaceResource{}
}

// SpaceResource defines the resource implementation.
type SpaceResource struct {
	client *ExtendedGqlClient
}

// ProjectResourceModel describes the resource data model.
type ProjectResourceModel struct {
	SpaceID  types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	OrgID    types.String `tfsdk:"org_id"`
	SpaceMrn types.String `tfsdk:"mrn"`
}

func (r *SpaceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_space"
}

func (r *SpaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Space resource",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the space.",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Id of the space. Must be globally unique.",
				Computed:            true,
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mrn": schema.StringAttribute{
				MarkdownDescription: "Mrn of the space.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"org_id": schema.StringAttribute{
				MarkdownDescription: "Id of the organization.",
				Required:            true,
			},
		},
	}
}

func (r *SpaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*mondoov1.Client)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *mondoov1.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = &ExtendedGqlClient{client}
}

func (r *SpaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource.
	payload, err := r.client.CreateSpace(ctx, data.OrgID.ValueString(), data.SpaceID.ValueString(), data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create space, got error: %s", err))
		return
	}

	// Save space mrn into the Terraform state.
	data.Name = types.StringValue(string(payload.Name))

	id, ok := payload.Id.(string)
	if ok {
		data.SpaceID = types.StringValue(id)
	}

	data.SpaceMrn = types.StringValue(string(payload.Mrn))

	// Write logs using the tflog package
	tflog.Trace(ctx, "created a space resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SpaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProjectResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// nothing to do here, we already have the data in the state

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SpaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProjectResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// ensure space id is not changed
	var stateSpaceID string
	req.State.GetAttribute(ctx, path.Root("id"), &stateSpaceID)

	var planSpaceID string
	req.Plan.GetAttribute(ctx, path.Root("id"), &planSpaceID)

	if stateSpaceID != planSpaceID {
		resp.Diagnostics.AddError(
			"Space ID cannot be changed",
			"Space ID cannot be changed",
		)
		return
	}

	// Do GraphQL request to API to update the resource.
	err := r.client.UpdateSpace(ctx, data.SpaceID.ValueString(), data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update space, got error: %s", err))
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SpaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProjectResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to delete the resource.
	err := r.client.DeleteSpace(ctx, data.SpaceID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete space, got error: %s", err))
		return
	}
}

func (r *SpaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	mrn := spacePrefix + req.ID
	spacePayload, err := r.client.GetSpace(ctx, mrn)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to retrieve space, got error: %s", err))
		return
	}

	model := ProjectResourceModel{
		SpaceID:  types.StringValue(spacePayload.Id),
		SpaceMrn: types.StringValue(spacePayload.Mrn),
		Name:     types.StringValue(spacePayload.Name),
		OrgID:    types.StringValue(spacePayload.Organization.Id),
	}

	resp.State.SetAttribute(ctx, path.Root("id"), model.SpaceID)
	resp.State.SetAttribute(ctx, path.Root("name"), model.Name)
	resp.State.SetAttribute(ctx, path.Root("org_id"), model.OrgID)
	resp.State.SetAttribute(ctx, path.Root("mrn"), model.SpaceMrn)
}
