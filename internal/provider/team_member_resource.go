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
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &TeamMemberResource{}

func NewTeamMemberResource() resource.Resource {
	return &TeamMemberResource{}
}

// TeamMemberResource defines the resource implementation.
type TeamMemberResource struct {
	client *ExtendedGqlClient
}

// TeamMemberResourceModel describes the resource data model.
type TeamMemberResourceModel struct {
	TeamMrn   types.String `tfsdk:"team_mrn"`
	Identity  types.String `tfsdk:"identity"`
	MemberMrn types.String `tfsdk:"member_mrn"`
}

func (r *TeamMemberResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_member"
}

func (r *TeamMemberResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `This resource manages team membership in Mondoo. It allows adding users to teams by email address or MRN. If the user does not yet have a Mondoo account, a pending membership is created.

**Example usage:**

` + "```hcl" + `
resource "mondoo_team" "example" {
  name      = "security-team"
  scope_mrn = mondoo_organization.example.mrn
}

resource "mondoo_team_member" "alice" {
  team_mrn = mondoo_team.example.mrn
  identity = "alice@example.com"
}
` + "```",

		Attributes: map[string]schema.Attribute{
			"team_mrn": schema.StringAttribute{
				MarkdownDescription: "MRN of the team to add the member to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"identity": schema.StringAttribute{
				MarkdownDescription: "Email address or MRN of the user to add to the team.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"member_mrn": schema.StringAttribute{
				MarkdownDescription: "MRN of the member. Empty if the user has not yet registered.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *TeamMemberResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *TeamMemberResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data TeamMemberResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	identity := mondoov1.String(data.Identity.ValueString())
	input := AddTeamMemberInput{
		TeamMrn:  mondoov1.String(data.TeamMrn.ValueString()),
		Identity: &identity,
	}

	payload, err := r.client.AddTeamMember(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to add team member, got error: %s", err))
		return
	}

	// Map response back to schema
	if payload.MemberMrn != nil {
		data.MemberMrn = types.StringValue(string(*payload.MemberMrn))
	} else {
		data.MemberMrn = types.StringValue("")
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamMemberResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data TeamMemberResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Get the team member from the API
	member, err := r.client.GetTeamMember(ctx, data.TeamMrn.ValueString(), data.Identity.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read team member, got error: %s", err))
		return
	}

	// If the member is not found, remove from state (drift detection)
	if member == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Map response back to schema
	if member.Mrn != nil {
		data.MemberMrn = types.StringValue(string(*member.Mrn))
	} else {
		data.MemberMrn = types.StringValue("")
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *TeamMemberResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	// Team members are immutable - both team_mrn and identity require replacement
	// This method should never be called due to RequiresReplace plan modifiers
	resp.Diagnostics.AddError(
		"Unexpected Update Call",
		"Team members cannot be updated. Both team_mrn and identity changes require replacement.",
	)
}

func (r *TeamMemberResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data TeamMemberResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	identity := mondoov1.String(data.Identity.ValueString())
	input := RemoveTeamMemberInput{
		TeamMrn:  mondoov1.String(data.TeamMrn.ValueString()),
		Identity: &identity,
	}

	err := r.client.RemoveTeamMember(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to remove team member, got error: %s", err))
		return
	}
}
