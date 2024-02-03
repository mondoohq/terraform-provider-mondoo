// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	mondoov1 "go.mondoo.com/mondoo-go"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*queryPackAssignmentResource)(nil)

func NewQueryPackAssigmentResource() resource.Resource {
	return &queryPackAssignmentResource{}
}

type queryPackAssignmentResource struct {
	client *ExtendedGqlClient
}

type queryPackAssigmentsResourceModel struct {
	// scope
	SpaceId types.String `tfsdk:"space_id"`

	// assigned query packs
	QueryPackMrns types.List `tfsdk:"querypacks"`

	// state
	State types.String `tfsdk:"state"`
}

func (r *queryPackAssignmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_querypack_assignment"
}

func (r *queryPackAssignmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo Space Identifier.",
				Required:            true,
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
				MarkdownDescription: "QueryPack Assignment State (enabled, disabled).",
				Default:             stringdefault.StaticString("enabled"),
				Computed:            true,
				Optional:            true,
			},
		},
	}
}

func (r *queryPackAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*mondoov1.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = &ExtendedGqlClient{client}
}

func (r *queryPackAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data queryPackAssigmentsResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource
	scopeMrn := ""
	if data.SpaceId.ValueString() != "" {
		scopeMrn = spacePrefix + data.SpaceId.ValueString()
	} else {
		resp.Diagnostics.AddError(
			"Either space_id needs to be set",
			"Either space_id needs to be set",
		)
		return
	}

	queryPackMrns := []string{}
	data.QueryPackMrns.ElementsAs(ctx, &queryPackMrns, false)

	state := data.State.ValueString()
	var err error
	// default action is active
	if state == "" || state == "enabled" {
		action := mondoov1.PolicyActionActive
		err = r.client.AssignPolicy(ctx, scopeMrn, action, queryPackMrns)
	} else if state == "disabled" {
		err = r.client.UnassignPolicy(ctx, scopeMrn, queryPackMrns)
	} else {
		resp.Diagnostics.AddError(
			"Invalid state: "+state,
			"Invalid state "+state+", use one of: enabled, disabled",
		)
		return
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating query pack assignment",
			fmt.Sprintf("Error creating query pack assignment: %s", err),
		)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *queryPackAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data queryPackAssigmentsResourceModel

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
	var data queryPackAssigmentsResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource
	scopeMrn := ""
	if data.SpaceId.ValueString() != "" {
		scopeMrn = spacePrefix + data.SpaceId.ValueString()
	} else {
		resp.Diagnostics.AddError(
			"Either space_id needs to be set",
			"Either space_id needs to be set",
		)
		return
	}

	queryPackMrns := []string{}
	data.QueryPackMrns.ElementsAs(ctx, &queryPackMrns, false)

	state := data.State.ValueString()
	var err error
	// default action is active
	if state == "" || state == "enabled" {
		action := mondoov1.PolicyActionActive
		err = r.client.AssignPolicy(ctx, scopeMrn, action, queryPackMrns)
	} else if state == "disabled" {
		err = r.client.UnassignPolicy(ctx, scopeMrn, queryPackMrns)
	} else {
		resp.Diagnostics.AddError(
			"Invalid state: "+state,
			"Invalid state "+state+", use one of: enabled, disabled",
		)
		return
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating query pack assignment",
			fmt.Sprintf("Error creating query pack assignment: %s", err),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *queryPackAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data queryPackAssigmentsResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource
	scopeMrn := ""
	if data.SpaceId.ValueString() != "" {
		scopeMrn = spacePrefix + data.SpaceId.ValueString()
	} else {
		resp.Diagnostics.AddError(
			"Either space_id needs to be set",
			"Either space_id needs to be set",
		)
		return
	}

	queryPackMrns := []string{}
	data.QueryPackMrns.ElementsAs(ctx, &queryPackMrns, false)

	err := r.client.UnassignPolicy(ctx, scopeMrn, queryPackMrns)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating query pack assignment",
			fmt.Sprintf("Error creating query pack assignment: %s", err),
		)
		return
	}
}
