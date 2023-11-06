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
	client *mondoov1.Client
}

// ProjectResourceModel describes the resource data model.
type ProjectResourceModel struct {
	SpaceID types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	OrgID   types.String `tfsdk:"org_id"`
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
				MarkdownDescription: "Space Name",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "Space identifier",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"org_id": schema.StringAttribute{
				MarkdownDescription: "Organization where the space is created",
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

	r.client = client
}

func (r *SpaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource.
	var createMutation struct {
		CreateSpace struct {
			Id   mondoov1.ID
			Mrn  mondoov1.String
			Name mondoov1.String
		} `graphql:"createSpace(input: $input)"`
	}
	createInput := mondoov1.CreateSpaceInput{
		Name:   mondoov1.String(data.Name.ValueString()),
		OrgMrn: mondoov1.String(orgPrefix + data.OrgID.ValueString()),
	}

	tflog.Trace(ctx, "CreateSpaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", createInput),
	})

	err := r.client.Mutate(ctx, &createMutation, createInput, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create space, got error: %s", err))
		return
	}

	// Save space mrn into the Terraform state.
	data.Name = types.StringValue(string(createMutation.CreateSpace.Name))
	id, ok := createMutation.CreateSpace.Id.(string)
	if ok {
		data.SpaceID = types.StringValue(id)
	}

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

	// Do GraphQL request to API to update the resource.
	var updateMutation struct {
		UpdateSpace struct {
			Space struct {
				Mrn  mondoov1.String
				Name mondoov1.String
			}
		} `graphql:"updateSpace(input: $input)"`
	}
	updateInput := mondoov1.UpdateSpaceInput{
		Mrn:  mondoov1.String(spacePrefix + data.SpaceID.ValueString()),
		Name: mondoov1.String(data.Name.ValueString()),
	}
	tflog.Trace(ctx, "UpdateSpaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", updateInput),
	})
	err := r.client.Mutate(ctx, &updateMutation, updateInput, nil)
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
	var deleteMutation struct {
		DeleteSpace mondoov1.String `graphql:"deleteSpace(spaceMrn: $spaceMrn)"`
	}
	variables := map[string]interface{}{
		"spaceMrn": mondoov1.ID(spacePrefix + data.SpaceID.ValueString()),
	}

	tflog.Trace(ctx, "DeleteSpaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", variables),
	})

	err := r.client.Mutate(ctx, &deleteMutation, nil, variables)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete space, got error: %s", err))
		return
	}
}

func (r *SpaceResource) readSpace(ctx context.Context, mrn string) (ProjectResourceModel, error) {
	var q struct {
		Space struct {
			Id           string
			Mrn          string
			Name         string
			Organization struct {
				Id string
			}
		} `graphql:"space(mrn: $mrn)"`
	}
	variables := map[string]interface{}{
		"mrn": mondoov1.String(mrn),
	}

	err := r.client.Query(ctx, &q, variables)
	if err != nil {
		return ProjectResourceModel{}, err
	}

	return ProjectResourceModel{
		SpaceID: types.StringValue(q.Space.Id),
		Name:    types.StringValue(q.Space.Name),
		OrgID:   types.StringValue(q.Space.Organization.Id),
	}, nil
}

func (r *SpaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	mrn := spacePrefix + req.ID
	model, err := r.readSpace(ctx, mrn)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to retrieve space, got error: %s", err))
		return
	}
	resp.State.SetAttribute(ctx, path.Root("id"), model.SpaceID)
	resp.State.SetAttribute(ctx, path.Root("name"), model.Name)
	resp.State.SetAttribute(ctx, path.Root("org_id"), model.OrgID)
}
