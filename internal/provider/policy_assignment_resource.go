// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*policyAssignmentResource)(nil)

func NewPolicyAssigmentResource() resource.Resource {
	return &policyAssignmentResource{}
}

type policyAssignmentResource struct {
	client *ExtendedGqlClient
}

type policyAssigmentsResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// assigned policies
	PolicyMrns types.List `tfsdk:"policies"`

	// state
	State types.String `tfsdk:"state"`
}

func (r *policyAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy_assignment"
}

func (r *policyAssignmentResource) Schema(_ context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo Space Identifier. If it is not provided, the provider space is used.",
				Optional:            true,
			},
			"policies": schema.ListAttribute{
				MarkdownDescription: "Policies to assign to the space.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Policy Assignment State (preview, enabled, disabled).",
				Default:             stringdefault.StaticString("enabled"),
				Computed:            true,
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("enabled", "disabled", "preview"),
				},
			},
		},
	}
}

func (r *policyAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *policyAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data policyAssigmentsResourceModel

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
	policyMrns := []string{}
	data.PolicyMrns.ElementsAs(ctx, &policyMrns, false)

	state := data.State.ValueString()
	tflog.Debug(ctx, "Creating policy assignment")
	// default action is active
	if state == "" || state == "enabled" {
		action := mondoov1.PolicyActionActive
		err = r.client.AssignPolicy(ctx, space.MRN(), action, policyMrns)
	} else if state == "preview" {
		action := mondoov1.PolicyActionIgnore
		err = r.client.AssignPolicy(ctx, space.MRN(), action, policyMrns)
	} else if state == "disabled" {
		err = r.client.UnassignPolicy(ctx, space.MRN(), policyMrns)
	} else {
		resp.Diagnostics.AddError(
			"Invalid state: "+state,
			"Invalid state "+state+", use one of: enabled, preview, disabled",
		)
		return
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating policy assignment",
			fmt.Sprintf("Error creating policy assignment: %s", err),
		)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *policyAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data policyAssigmentsResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *policyAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data policyAssigmentsResourceModel

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
	policyMrns := []string{}
	data.PolicyMrns.ElementsAs(ctx, &policyMrns, false)

	state := data.State.ValueString()
	tflog.Debug(ctx, "Updating policy assignment")
	// default action is active
	if state == "" || state == "enabled" {
		action := mondoov1.PolicyActionActive
		err = r.client.AssignPolicy(ctx, space.MRN(), action, policyMrns)
	} else if state == "preview" {
		action := mondoov1.PolicyActionIgnore
		err = r.client.AssignPolicy(ctx, space.MRN(), action, policyMrns)
	} else if state == "disabled" {
		err = r.client.UnassignPolicy(ctx, space.MRN(), policyMrns)
	} else {
		resp.Diagnostics.AddError(
			"Invalid state: "+state,
			"Invalid state "+state+", use one of: enabled, preview, disabled",
		)
		return
	}

	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating policy assignment",
			fmt.Sprintf("Error creating policy assignment: %s", err),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *policyAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data policyAssigmentsResourceModel

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

	// Do GraphQL request to API to create the resource
	policyMrns := []string{}
	data.PolicyMrns.ElementsAs(ctx, &policyMrns, false)

	tflog.Debug(ctx, "Deleting policy assignment")
	// no matter the state, we unassign the policies
	err = r.client.UnassignPolicy(ctx, space.MRN(), policyMrns)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating policy assignment",
			fmt.Sprintf("Error creating policy assignment: %s", err),
		)
		return
	}
}
