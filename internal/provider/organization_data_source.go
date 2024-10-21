// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &OrganizationDataSource{}

func NewOrganizationDataSource() datasource.DataSource {
	return &OrganizationDataSource{}
}

// OrganizationDataSource defines the data source implementation.
type OrganizationDataSource struct {
	client *ExtendedGqlClient
}

// OrganizationDataSourceModel describes the data source data model.
type OrganizationDataSourceModel struct {
	OrgID  types.String `tfsdk:"id"`
	OrgMrn types.String `tfsdk:"mrn"`
	Name   types.String `tfsdk:"name"`
}

func (d *OrganizationDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (d *OrganizationDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Organization data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Organization ID",
				Computed:            true,
				Optional:            true,
				Validators: []validator.String{
					// Validate only this attribute or other_attr is configured.
					stringvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("mrn"),
					}...),
				},
			},
			"mrn": schema.StringAttribute{
				MarkdownDescription: "Organization MRN",
				Computed:            true,
				Optional:            true,
				Validators: []validator.String{
					// Validate only this attribute or other_attr is configured.
					stringvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("id"),
					}...),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Organization name",
				Computed:            true,
			},
		},
	}
}

func (d *OrganizationDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *mondoov1.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

func (d *OrganizationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data OrganizationDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// we fetch the organization id from the service account
	orgMrn := ""
	if data.OrgMrn.ValueString() != "" {
		orgMrn = data.OrgMrn.ValueString()
	} else if data.OrgID.ValueString() != "" {
		orgMrn = orgPrefix + data.OrgID.ValueString()
	}

	if orgMrn == "" {
		resp.Diagnostics.AddError("Invalid Configuration", "Either `id` or `mrn` must be set")
		return
	}

	payload, err := d.client.GetOrganization(ctx, orgMrn)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch organization, got error: %s", err))
		return
	}

	data.OrgID = types.StringValue(payload.Id)
	data.OrgMrn = types.StringValue(payload.Mrn)
	data.Name = types.StringValue(payload.Name)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
