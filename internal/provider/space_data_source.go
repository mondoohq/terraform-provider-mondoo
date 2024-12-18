// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
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
			},
			"mrn": schema.StringAttribute{
				MarkdownDescription: "Space MRN",
				Computed:            true,
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

	client, ok := req.ProviderData.(*ExtendedGqlClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *mondoov1.Client. Got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	d.client = client
}

func (d *SpaceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data SpaceDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Compute and validate the space
	space, err := d.client.ComputeSpace(data.SpaceID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}

	ctx = tflog.SetField(ctx, "space_mrn", space.MRN())

	tflog.Debug(ctx, "Fetching space information")
	payload, err := d.client.GetSpace(ctx, space.MRN())
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to fetch space. Got error: %s", err),
		)
		return
	}

	data.SpaceID = types.StringValue(payload.Id)
	data.SpaceMrn = types.StringValue(payload.Mrn)
	data.Name = types.StringValue(payload.Name)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
