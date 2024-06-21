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
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &SpaceDataSource{}

func NewSpaceDataSource() datasource.DataSource {
	return &SpaceDataSource{}
}

// SpaceDataSource defines the data source implementation.
type SpaceDataSource struct {
	client *ExtendedGqlClient
}

// SpaceDataSourceModel describes the data source data model.
type SpaceDataSourceModel struct {
	SpaceID  types.String `tfsdk:"id"`
	SpaceMrn types.String `tfsdk:"mrn"`
	Name     types.String `tfsdk:"name"`
}

func (d *SpaceDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_space"
}

func (d *SpaceDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Space data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "Space ID",
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
				MarkdownDescription: "Space MRN",
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
				MarkdownDescription: "Space name",
				Computed:            true,
			},
		},
	}
}

func (d *SpaceDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*mondoov1.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *mondoov1.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = &ExtendedGqlClient{client}
}

func (d *SpaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SpaceDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// we fetch the organization id from the service account
	spaceMrn := ""
	if data.SpaceMrn.ValueString() != "" {
		spaceMrn = data.SpaceMrn.ValueString()
	} else if data.SpaceID.ValueString() != "" {
		spaceMrn = spacePrefix + data.SpaceID.ValueString()
	}

	if spaceMrn == "" {
		resp.Diagnostics.AddError("Invalid Configuration", "Either `id` or `mrn` must be set")
		return
	}

	payload, err := d.client.GetSpace(ctx, spaceMrn)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to fetch organization, got error: %s", err))
		return
	}

	data.SpaceID = types.StringValue(payload.Id)
	data.SpaceMrn = types.StringValue(payload.Mrn)
	data.Name = types.StringValue(payload.Name)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
