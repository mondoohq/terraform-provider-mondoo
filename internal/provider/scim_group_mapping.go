// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var _ resource.Resource = (*policyAssignmentResource)(nil)

func NewScimGroupMappingResource() resource.Resource {
	return &scimGroupMappingResource{}
}

type scimGroupMappingResource struct {
	client *ExtendedGqlClient
}

type scimGroupMappingResourceModel struct {
	// scope
	OrgID types.String `tfsdk:"org_id"`
	Group types.String `tfsdk:"group"`

	// mapped org and spaces
	Mappings []scimGroupMappingResourceMappingModel `tfsdk:"mappings"`
}

type scimGroupMappingResourceMappingModel struct {
	IamRole  types.String `tfsdk:"iam_role"`
	SpaceMrn types.String `tfsdk:"space_mrn"`
	OrgMrn   types.String `tfsdk:"org_mrn"`
}

func (r *scimGroupMappingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scim_group_mapping"
}

func (r *scimGroupMappingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
		This resource provides SCIM 2.0 Group Mapping. It allows the mapping of SCIM 2.0 groups to Mondoo organization or spaces and IAM roles.
		`,
		Attributes: map[string]schema.Attribute{
			"org_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo Organization Identifier.",
				Required:            true,
			},
			"group": schema.StringAttribute{
				MarkdownDescription: "SCIM 2.0 Group Display Name.",
				Required:            true,
			},
			// https://developer.hashicorp.com/terraform/plugin/framework/handling-data/attributes/list-nested
			"mappings": schema.ListNestedAttribute{
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"iam_role": schema.StringAttribute{
							Required: true,
						},
						"space_mrn": schema.StringAttribute{
							Optional: true,
						},
						"org_mrn": schema.StringAttribute{
							Optional: true,
						},
					},
				},
				Required: true,
			},
		},
	}
}

func (r *scimGroupMappingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *scimGroupMappingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data scimGroupMappingResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource
	mappings := []mondoov1.ScimGroupMapping{}

	for i := range data.Mappings {
		m := data.Mappings[i]
		mapping := mondoov1.ScimGroupMapping{
			IamRole: mondoov1.String(m.IamRole.ValueString()),
		}

		if !m.SpaceMrn.IsNull() {
			mapping.SpaceMrn = mondoov1.NewStringPtr(mondoov1.String(m.SpaceMrn.ValueString()))
		}

		if !m.OrgMrn.IsNull() {
			mapping.OrgMrn = mondoov1.NewStringPtr(mondoov1.String(m.OrgMrn.ValueString()))
		}

		mappings = append(mappings, mapping)
	}

	err := r.client.SetScimGroupMapping(ctx, orgPrefix+data.OrgID.ValueString(), data.Group.ValueString(), mappings)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating SCIM group mapping",
			fmt.Sprintf("Error creating SCIM group mapping: %s", err),
		)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *scimGroupMappingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data scimGroupMappingResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *scimGroupMappingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data scimGroupMappingResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource
	mappings := []mondoov1.ScimGroupMapping{}

	for i := range data.Mappings {
		m := data.Mappings[i]
		mapping := mondoov1.ScimGroupMapping{
			IamRole: mondoov1.String(m.IamRole.ValueString()),
		}

		if !m.SpaceMrn.IsNull() {
			mapping.SpaceMrn = mondoov1.NewStringPtr(mondoov1.String(m.SpaceMrn.ValueString()))
		}

		if !m.OrgMrn.IsNull() {
			mapping.OrgMrn = mondoov1.NewStringPtr(mondoov1.String(m.OrgMrn.ValueString()))
		}

		mappings = append(mappings, mapping)
	}

	err := r.client.SetScimGroupMapping(ctx, orgPrefix+data.OrgID.ValueString(), data.Group.ValueString(), mappings)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating SCIM group mapping",
			fmt.Sprintf("Error creating SCIM group mapping: %s", err),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *scimGroupMappingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data scimGroupMappingResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource

	// we intentionally set an empty mapping to remove the mapping
	mappings := []mondoov1.ScimGroupMapping{}
	err := r.client.SetScimGroupMapping(ctx, orgPrefix+data.OrgID.ValueString(), data.Group.ValueString(), mappings)
	if err != nil {
		resp.Diagnostics.AddError(
			"Error creating SCIM group mapping",
			fmt.Sprintf("Error creating SCIM group mapping: %s", err),
		)
		return
	}

}
