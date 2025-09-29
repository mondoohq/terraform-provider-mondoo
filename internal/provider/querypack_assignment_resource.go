// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*queryPackAssignmentResource)(nil)

func NewQueryPackAssignmentResource() resource.Resource {
	return &queryPackAssignmentResource{}
}

type queryPackAssignmentResource struct {
	client *ExtendedGqlClient
}

type queryPackAssignmentsResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// assigned query packs
	QueryPackMrns types.List `tfsdk:"querypacks"`

	// state
	State types.String `tfsdk:"state"`
}

func (r *queryPackAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_querypack_assignment"
}

func (r *queryPackAssignmentResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo space identifier. If there is no space ID, the provider space is used.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"querypacks": schema.ListAttribute{
				MarkdownDescription: "QueryPacks to assign to the space.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "QueryPack Assignment State (enabled or disabled).",
				Default:             stringdefault.StaticString("enabled"),
				Computed:            true,
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("enabled", "disabled"),
				},
			},
		},
	}
}

func (r *queryPackAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client. Got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *queryPackAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data queryPackAssignmentsResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Compute and validate the space
	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "space_mrn", space.MRN())

	// Do GraphQL request to API to create the resource
	queryPackMrns := []string{}
	data.QueryPackMrns.ElementsAs(ctx, &queryPackMrns, false)

	state := data.State.ValueString()
	tflog.Debug(ctx, "Creating query pack assignment")
	// default action is active
	switch state {
	case "", "enabled":
		action := mondoov1.PolicyActionActive
		err = r.client.AssignPolicy(ctx, space.MRN(), action, queryPackMrns)
	case "disabled":
		err = r.client.UnassignPolicy(ctx, space.MRN(), queryPackMrns)
	default:
		resp.Diagnostics.AddError(
			"Invalid state: "+state,
			"Invalid state "+state+", use one of: enabled, disabled",
		)
		return
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating query pack assignment",
			fmt.Sprintf(
				"Error creating query pack assignment: %s\nSpace: %s\nQueryPacks: %s",
				err, space.MRN(), strings.Join(queryPackMrns, "\n"),
			),
		)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *queryPackAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data queryPackAssignmentsResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *queryPackAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data queryPackAssignmentsResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Compute and validate the space
	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "space_mrn", space.MRN())

	// Do GraphQL request to API to create the resource
	queryPackMrns := []string{}
	data.QueryPackMrns.ElementsAs(ctx, &queryPackMrns, false)

	state := data.State.ValueString()
	tflog.Debug(ctx, "Updating query pack assignment")
	// default action is active
	switch state {
	case "", "enabled":
		action := mondoov1.PolicyActionActive
		err = r.client.AssignPolicy(ctx, space.MRN(), action, queryPackMrns)
	case "disabled":
		err = r.client.UnassignPolicy(ctx, space.MRN(), queryPackMrns)
	default:
		resp.Diagnostics.AddError(
			"Invalid state: "+state,
			"Invalid state "+state+". Valid states are enabled and disabled",
		)
		return
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating query pack assignment",
			fmt.Sprintf(
				"Error creating query pack assignment: %s\nSpace: %s\nQueryPacks: %s",
				err, space.MRN(), strings.Join(queryPackMrns, "\n"),
			),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *queryPackAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data queryPackAssignmentsResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Compute and validate the space
	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "space_mrn", space.MRN())

	// Do GraphQL request to API to delete the resource
	queryPackMrns := []string{}
	data.QueryPackMrns.ElementsAs(ctx, &queryPackMrns, false)

	tflog.Debug(ctx, "Deleting query pack assignment")
	err = r.client.UnassignPolicy(ctx, space.MRN(), queryPackMrns)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating query pack assignment",
			fmt.Sprintf("Error creating query pack assignment: %s", err),
		)
		return
	}
}
