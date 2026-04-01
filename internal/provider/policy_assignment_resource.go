// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = (*policyAssignmentResource)(nil)

func NewPolicyAssignmentResource() resource.Resource {
	return &policyAssignmentResource{}
}

type policyAssignmentResource struct {
	client *ExtendedGqlClient
}

type policyAssignmentsResourceModel struct {
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
				MarkdownDescription: "Mondoo space identifier. If there is no space ID, the provider space is used.",
				Optional:            true,
			},
			"policies": schema.ListAttribute{
				MarkdownDescription: "Policies to assign to the space.",
				ElementType:         types.StringType,
				Required:            true,
				Validators:          []validator.List{listvalidator.SizeAtLeast(1)},
			},
			"state": schema.StringAttribute{
				MarkdownDescription: "Policy assignment state (preview, enabled, or disabled).",
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
			fmt.Sprintf("Expected *http.Client. Got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *policyAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data policyAssignmentsResourceModel

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
	switch state {
	case "", "enabled":
		action := mondoov1.PolicyActionActive
		err = r.client.AssignPolicy(ctx, space.MRN(), action, policyMrns)
	case "preview":
		action := mondoov1.PolicyActionIgnore
		err = r.client.AssignPolicy(ctx, space.MRN(), action, policyMrns)
	case "disabled":
		err = r.client.UnassignPolicy(ctx, space.MRN(), policyMrns)
	default:
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
	var data policyAssignmentsResourceModel

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

	// Fetch active policies from API
	activePolicies, err := r.client.GetActivePolicies(ctx, space.MRN())
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch active policies", err.Error())
		return
	}

	// Build lookup map: policyMrn -> action, filtered by assignedScope
	policyActions := make(map[string]string)
	for _, p := range activePolicies {
		if string(p.AssignedScope) == space.MRN() {
			policyActions[string(p.Mrn)] = string(p.Action)
		}
	}

	// Check the actual state of each configured policy
	policyMrns := []string{}
	data.PolicyMrns.ElementsAs(ctx, &policyMrns, false)

	configuredState := data.State.ValueString()
	allMatch := true
	for _, mrn := range policyMrns {
		action, found := policyActions[mrn]
		var actualState string
		if !found {
			actualState = "disabled"
		} else {
			switch action {
			case "ACTIVE":
				actualState = "enabled"
			case "IGNORE":
				actualState = "preview"
			default:
				actualState = "disabled"
			}
		}
		if actualState != configuredState {
			allMatch = false
			// Report the actual state of this policy so Terraform sees the drift
			data.State = types.StringValue(actualState)
			break
		}
	}

	if allMatch {
		// All policies match the configured state, no drift
		data.State = types.StringValue(configuredState)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *policyAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data policyAssignmentsResourceModel

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
	switch state {
	case "", "enabled":
		action := mondoov1.PolicyActionActive
		err = r.client.AssignPolicy(ctx, space.MRN(), action, policyMrns)
	case "preview":
		action := mondoov1.PolicyActionIgnore
		err = r.client.AssignPolicy(ctx, space.MRN(), action, policyMrns)
	case "disabled":
		err = r.client.UnassignPolicy(ctx, space.MRN(), policyMrns)
	default:
		resp.Diagnostics.AddError(
			"Invalid state: "+state,
			"Invalid state "+state+". Valid states are enabled, preview, and disabled",
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
	var data policyAssignmentsResourceModel

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
