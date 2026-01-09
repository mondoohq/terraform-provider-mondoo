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
	"go.mondoo.com/terraform-provider-mondoo/internal/customtypes"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &IAMBindingResource{}

func NewIAMBindingResource() resource.Resource {
	return &IAMBindingResource{}
}

// IAMBindingResource defines the resource implementation.
type IAMBindingResource struct {
	client *ExtendedGqlClient
}

// IAMBindingResourceModel describes the resource data model.
type IAMBindingResourceModel struct {
	IdentityMrn types.String `tfsdk:"identity_mrn"`
	ResourceMrn types.String `tfsdk:"resource_mrn"`
	Roles       types.List   `tfsdk:"roles"`
}

func (r *IAMBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_binding"
}

func (r *IAMBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
This resource manages IAM role bindings in Mondoo. It assigns roles to identity principals (users, service accounts, or teams) on specific resources (organizations, spaces, workspaces, etc.).

## Example Usage

` + "```hcl" + `
# Grant a team editor permissions on a space
resource "mondoo_iam_binding" "team_permissions" {
  identity_mrn = mondoo_team.security_team.mrn
  resource_mrn = mondoo_space.production.mrn
  roles        = ["//iam.api.mondoo.app/roles/editor"]
}

# Grant a team viewer permissions on a workspace
resource "mondoo_iam_binding" "security_team_permissions" {
  identity_mrn = mondoo_team.team_1.mrn
  resource_mrn = mondoo_workspace.my_workspace.mrn
  roles        = ["//iam.api.mondoo.app/roles/viewer"]
}
` + "```",

		Attributes: map[string]schema.Attribute{
			"identity_mrn": schema.StringAttribute{
				MarkdownDescription: "MRN of the identity principal (team, user, or service account) to grant roles to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"resource_mrn": schema.StringAttribute{
				MarkdownDescription: "MRN of the resource (organization, space, workspace, etc.) to grant access to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"roles": schema.ListAttribute{
				MarkdownDescription: `List of role names to assign to the identity on the resource. Can be specified as short names (e.g. "editor") or full MRNs (e.g. "//iam.api.mondoo.app/roles/editor"). Available roles: integrations-manager, sla-manager, policy-manager, policy-editor, ticket-manager, ticket-creator, exceptions-requester, query-pack-manager, query-pack-editor, viewer, editor, owner.`,
				Required:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.List{
					RoleListNormalizerModifier(),
				},
			},
		},
	}
}

func (r *IAMBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *IAMBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data IAMBindingResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Convert roles to the format expected by the API
	var roleInputs []RoleInput
	var roleStrings []string
	data.Roles.ElementsAs(ctx, &roleStrings, false)
	for _, role := range roleStrings {
		// Normalize role names to full MRNs
		normalizedRole := customtypes.NormalizeRoleMRN(role)
		roleInputs = append(roleInputs, RoleInput{
			Mrn: mondoov1.String(normalizedRole),
		})
	}

	// Set roles using the setRoles mutation
	input := SetRolesInput{
		ScopeMrn: mondoov1.String(data.ResourceMrn.ValueString()),
		Updates: []SetRoleInput{
			{
				EntityMrn: mondoov1.String(data.IdentityMrn.ValueString()),
				Roles:     roleInputs,
			},
		},
	}

	_, err := r.client.SetRoles(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create IAM binding, got error: %s", err))
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IAMBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IAMBindingResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Query current roles from the API
	rolesPayload, err := r.client.GetRoles(ctx, data.IdentityMrn.ValueString(), data.ResourceMrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read IAM binding, got error: %s", err))
		return
	}

	// Filter out implicit roles (org-member, space-member) that are automatically added
	var explicitRoles []string
	for _, role := range rolesPayload.Roles {
		roleStr := string(role)
		// Skip implicit membership roles
		if roleStr != "//iam.api.mondoo.app/roles/org-member" && roleStr != "//iam.api.mondoo.app/roles/space-member" {
			explicitRoles = append(explicitRoles, roleStr)
		}
	}

	// Convert to Terraform list type
	rolesList, diags := types.ListValueFrom(ctx, types.StringType, explicitRoles)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data.Roles = rolesList

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IAMBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data IAMBindingResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Convert roles to the format expected by the API
	var roleInputs []RoleInput
	var roleStrings []string
	data.Roles.ElementsAs(ctx, &roleStrings, false)
	for _, role := range roleStrings {
		// Normalize role names to full MRNs
		normalizedRole := customtypes.NormalizeRoleMRN(role)
		roleInputs = append(roleInputs, RoleInput{
			Mrn: mondoov1.String(normalizedRole),
		})
	}

	// Update roles using the setRoles mutation
	input := SetRolesInput{
		ScopeMrn: mondoov1.String(data.ResourceMrn.ValueString()),
		Updates: []SetRoleInput{
			{
				EntityMrn: mondoov1.String(data.IdentityMrn.ValueString()),
				Roles:     roleInputs,
			},
		},
	}

	_, err := r.client.SetRoles(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update IAM binding, got error: %s", err))
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IAMBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data IAMBindingResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Remove roles by setting an empty role list
	input := SetRolesInput{
		ScopeMrn: mondoov1.String(data.ResourceMrn.ValueString()),
		Updates: []SetRoleInput{
			{
				EntityMrn: mondoov1.String(data.IdentityMrn.ValueString()),
				Roles:     []RoleInput{}, // Empty list removes all roles
			},
		},
	}

	_, err := r.client.SetRoles(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete IAM binding, got error: %s", err))
		return
	}
}
