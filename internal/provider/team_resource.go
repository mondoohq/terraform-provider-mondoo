// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TeamResource{}

func NewTeamResource() resource.Resource {
	return &TeamResource{}
}

// TeamResource defines the resource implementation.
type TeamResource struct {
	client *ExtendedGqlClient
}

// TeamResourceModel describes the resource data model.
type TeamResourceModel struct {
	Id          types.String `tfsdk:"id"`
	Mrn         types.String `tfsdk:"mrn"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	ScopeMrn    types.String `tfsdk:"scope_mrn"`
	CreatedAt   types.String `tfsdk:"created_at"`
	UpdatedAt   types.String `tfsdk:"updated_at"`
}

func (r *TeamResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (r *TeamResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
This resource manages Mondoo Teams

## Example Usage

` + "```hcl" + `
resource "mondoo_team" "security_team" {
  name        = "security-team"
  description = "Team responsible for security policies and compliance"
  scope_mrn   = data.mondoo_organization.current.mrn
}

# Grant team permissions using existing IAM binding resource
resource "mondoo_iam_binding" "security_team_permissions" {
  identity_mrn = mondoo_team.security_team.mrn
  resource_mrn = mondoo_space.production.mrn
  roles        = ["//iam.api.mondoo.app/roles/editor"]
}
` + "```",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "ID of the team. If not provided, it will be auto-generated.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mrn": schema.StringAttribute{
				MarkdownDescription: "Mondoo Resource Name (MRN) of the team.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the team.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the team.",
				Optional:            true,
			},
			"scope_mrn": schema.StringAttribute{
				MarkdownDescription: "MRN of the scope (organization or space) that owns this team.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp when the team was created.",
				Computed:            true,
			},
			"updated_at": schema.StringAttribute{
				MarkdownDescription: "RFC 3339 timestamp when the team was last updated.",
				Computed:            true,
			},
		},
	}
}

func (r *TeamResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ExtendedGqlClient. Got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *TeamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TeamResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create the team using GraphQL mutation
	input := CreateTeamInput{
		Name:        mondoov1.String(data.Name.ValueString()),
		Description: mondoov1.NewStringPtr(mondoov1.String(data.Description.ValueString())),
		ScopeMrn:    mondoov1.String(data.ScopeMrn.ValueString()),
	}

	// Set ID if provided
	if !data.Id.IsNull() && !data.Id.IsUnknown() {
		input.Id = mondoov1.NewStringPtr(mondoov1.String(data.Id.ValueString()))
	}

	team, err := r.client.CreateTeam(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating team",
			fmt.Sprintf("Could not create team: %s", err),
		)
		return
	}

	// Update the model with response data
	data.Id = types.StringValue(string(team.Id))
	data.Mrn = types.StringValue(string(team.Mrn))
	data.Name = types.StringValue(string(team.Name))
	data.Description = types.StringValue(string(*team.Description))
	data.ScopeMrn = types.StringValue(string(team.ScopeMrn))
	data.CreatedAt = types.StringValue(string(team.CreatedAt))
	data.UpdatedAt = types.StringValue(string(team.UpdatedAt))

	tflog.Trace(ctx, "created team", map[string]interface{}{
		"mrn": data.Mrn.ValueString(),
	})

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TeamResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get team from API
	team, err := r.client.GetTeam(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error reading team",
			fmt.Sprintf("Could not read team %s: %s", data.Mrn.ValueString(), err),
		)
		return
	}

	// Update model with current state
	data.Id = types.StringValue(string(team.Id))
	data.Name = types.StringValue(string(team.Name))
	if team.Description != nil {
		data.Description = types.StringValue(string(*team.Description))
	} else {
		data.Description = types.StringNull()
	}
	data.ScopeMrn = types.StringValue(string(team.ScopeMrn))
	data.CreatedAt = types.StringValue(string(team.CreatedAt))
	data.UpdatedAt = types.StringValue(string(team.UpdatedAt))

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data TeamResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Update the team using GraphQL mutation
	team, err := r.client.UpdateTeam(ctx, UpdateTeamInput{
		Mrn:         mondoov1.String(data.Mrn.ValueString()),
		Name:        mondoov1.String(data.Name.ValueString()),
		Description: mondoov1.NewStringPtr(mondoov1.String(data.Description.ValueString())),
	})
	if err != nil {
		resp.Diagnostics.AddError(
			"Error updating team",
			fmt.Sprintf("Could not update team %s: %s", data.Mrn.ValueString(), err),
		)
		return
	}

	// Update the model with response data
	data.Name = types.StringValue(string(team.Name))
	if team.Description != nil {
		data.Description = types.StringValue(string(*team.Description))
	} else {
		data.Description = types.StringNull()
	}
	data.UpdatedAt = types.StringValue(string(team.UpdatedAt))

	tflog.Trace(ctx, "updated team", map[string]interface{}{
		"mrn": data.Mrn.ValueString(),
	})

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TeamResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the team using GraphQL mutation
	err := r.client.DeleteTeam(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Error deleting team",
			fmt.Sprintf("Could not delete team %s: %s", data.Mrn.ValueString(), err),
		)
		return
	}

	tflog.Trace(ctx, "deleted team", map[string]interface{}{
		"mrn": data.Mrn.ValueString(),
	})
}
