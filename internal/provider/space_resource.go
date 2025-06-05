// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^([a-zA-Z \-'_]|\d){2,30}$`),
						"must contain 2 to 30 characters, where each character can be a letter (uppercase or lowercase), a space, a dash, an underscore, or a digit",
					),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "ID of the space. Must be globally unique. If the provider has a space configured and this field is empty, the provider space is used.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-z\d]([\d-_]|[a-z]){2,48}[a-z\d]$`),
						"must contain 4 to 50 digits, dashes, underscores, or lowercase letters, and ending with either a lowercase letter or a digit",
					),
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
				MarkdownDescription: "ID of the organization.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-z]([\d-_]|[a-z]){6,35}[a-z\d]$`),
						"must contain 6 to 35 digits, dashes, underscores, or lowercase letters, and ending with either a lowercase letter or a digit",
					),
				},
			},
		},
	}
}

func (r *SpaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *mondoov1.Client. Got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *SpaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Compute and validate the space
	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		// we do not fail if there the user doesn't specify an id
		// because we are creating one, still log the error
		tflog.Debug(ctx, err.Error())
	}

	// Do GraphQL request to API to create the resource.
	payload, err := r.client.CreateSpace(ctx,
		data.OrgID.ValueString(),
		space.ID(),
		data.Name.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create space. Got error: %s", err),
			)
		return
	}

	// Save space mrn into the Terraform state.
	data.Name = types.StringValue(string(payload.Name))

	id, ok := payload.Id.(string)
	if ok {
		data.SpaceID = types.StringValue(id)
		ctx = tflog.SetField(ctx, "space_id", data.SpaceID)
	}

	data.SpaceMrn = types.StringValue(string(payload.Mrn))
	ctx = tflog.SetField(ctx, "space_mrn", data.SpaceMrn)

	// Write logs using the tflog package
	tflog.Debug(ctx, "Created a space resource")

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

	// Compute and validate the space
	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		// we do not fail if there the user doesn't specify an id
		// because we are creating one, still log the error
		tflog.Debug(ctx, err.Error())
	}
	ctx = tflog.SetField(ctx, "computed_space_id", space.ID())

	// ensure space id is not changed
	var planSpaceID string
	req.Plan.GetAttribute(ctx, path.Root("id"), &planSpaceID)

	if space.ID() != planSpaceID {
		resp.Diagnostics.AddError(
			"Space ID cannot be changed",
			"Note that the Mondoo space can be configured at the resource or provider level.",
		)
		return
	}
	ctx = tflog.SetField(ctx, "space_id_from_plan", planSpaceID)

	// Do GraphQL request to API to update the resource.
	tflog.Debug(ctx, "Updating space")
	err = r.client.UpdateSpace(ctx, planSpaceID, data.Name.ValueString())
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error", fmt.Sprintf("Unable to update space. Got error: %s", err))
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
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to delete space. Got error: %s", err),
			)
		return
	}
}

func (r *SpaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	mrn := spacePrefix + req.ID
	spacePayload, err := r.client.GetSpace(ctx, mrn)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to retrieve space. Got error: %s", err),
			)
		return
	}

	model := ProjectResourceModel{
		SpaceID:  types.StringValue(spacePayload.Id),
		SpaceMrn: types.StringValue(spacePayload.Mrn),
		Name:     types.StringValue(spacePayload.Name),
		OrgID:    types.StringValue(spacePayload.Organization.Id),
	}

	resp.State.Set(ctx, &model)
}
