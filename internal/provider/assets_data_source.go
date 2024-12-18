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

var _ datasource.DataSource = (*assetsDataSource)(nil)

func NewAssetsDataSource() datasource.DataSource {
	return &assetsDataSource{}
}

type assetsDataSource struct {
	client *ExtendedGqlClient
}

type assetsDataSourceModel struct {
	Id           types.String       `tfsdk:"id"`
	Mrn          types.String       `tfsdk:"mrn"`
	State        types.String       `tfsdk:"state"`
	Name         types.String       `tfsdk:"name"`
	UpdatedAt    types.String       `tfsdk:"updated_at"`
	ReferenceIDs []types.String     `tfsdk:"reference_ids"`
	AssetType    types.String       `tfsdk:"asset_type"`
	Annotations  []annotationsModel `tfsdk:"annotations"`
	Score        scoreModel         `tfsdk:"score"`
}

type annotationsModel struct {
	Key   string `tfsdk:"key"`
	Value string `tfsdk:"value"`
}

type scoreModel struct {
	Grade types.String `tfsdk:"grade"`
	Value types.Int64  `tfsdk:"value"`
}

type spaceAssetDataSourceModel struct {
	SpaceID  types.String            `tfsdk:"space_id"`
	SpaceMrn types.String            `tfsdk:"space_mrn"`
	Assets   []assetsDataSourceModel `tfsdk:"assets"`
}

func (d *assetsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_assets"
}

func (d *assetsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The asset data source allows you to fetch assets from a space.",
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "The unique identifier of the space.",
				Computed:            true,
				Optional:            true,
				Validators: []validator.String{
					// Validate only this attribute or other_attr is configured.
					stringvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("space_mrn"),
					}...),
				},
			},
			"space_mrn": schema.StringAttribute{
				MarkdownDescription: "The unique Mondoo Resource Name (MRN) of the space.",
				Computed:            true,
				Optional:            true,
				Validators: []validator.String{
					// Validate only this attribute or other_attr is configured.
					stringvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("space_id"),
					}...),
				},
			},
			"assets": schema.ListNestedAttribute{
				MarkdownDescription: "The list of assets in the space.",
				Computed:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.StringAttribute{
							MarkdownDescription: "The unique identifier of the asset.",
							Computed:            true,
						},
						"mrn": schema.StringAttribute{
							MarkdownDescription: "The unique Mondoo Resource Name (MRN) of the asset.",
							Computed:            true,
						},
						"state": schema.StringAttribute{
							MarkdownDescription: "The current state of the asset.",
							Computed:            true,
						},
						"name": schema.StringAttribute{
							MarkdownDescription: "The name of the asset.",
							Computed:            true,
						},
						"updated_at": schema.StringAttribute{
							MarkdownDescription: "The timestamp when the asset was last updated.",
							Computed:            true,
						},
						"reference_ids": schema.ListAttribute{
							MarkdownDescription: "The reference IDs of the asset.",
							Computed:            true,
							ElementType:         types.StringType,
						},
						"asset_type": schema.StringAttribute{
							MarkdownDescription: "The type of the asset.",
							Computed:            true,
						},
						"annotations": schema.ListNestedAttribute{
							MarkdownDescription: "The annotations/tags of the asset.",
							Computed:            true,
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"key": schema.StringAttribute{
										MarkdownDescription: "The key of the annotation.",
										Computed:            true,
									},
									"value": schema.StringAttribute{
										MarkdownDescription: "The value of the annotation.",
										Computed:            true,
									},
								},
							},
						},
						"score": schema.SingleNestedAttribute{
							MarkdownDescription: "The overall score of the asset.",
							Computed:            true,
							Attributes: map[string]schema.Attribute{
								"grade": schema.StringAttribute{
									MarkdownDescription: "The grade of the asset.",
									Computed:            true,
								},
								"value": schema.Int64Attribute{
									MarkdownDescription: "The score value of the asset.",
									Computed:            true,
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *assetsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *assetsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data spaceAssetDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

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

	// Read API call logic
	assets, err := d.client.GetAssets(ctx, spaceMrn)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch assets", err.Error())
		return
	}

	// Map API response to the model
	data.Assets = make([]assetsDataSourceModel, len(assets.Edges))
	for i, asset := range assets.Edges {

		referenceIDs := make([]types.String, len(asset.Node.ReferenceIDs))
		for j, refID := range asset.Node.ReferenceIDs {
			referenceIDs[j] = types.StringValue(refID)
		}

		annotations := make([]annotationsModel, len(asset.Node.Annotations))
		for j, annotation := range asset.Node.Annotations {
			annotations[j] = *convertToAnnotationsModel(annotation)
		}

		data.Assets[i] = assetsDataSourceModel{
			Id:           types.StringValue(asset.Node.Id),
			Mrn:          types.StringValue(asset.Node.Mrn),
			State:        types.StringValue(asset.Node.State),
			Name:         types.StringValue(asset.Node.Name),
			UpdatedAt:    types.StringValue(asset.Node.UpdatedAt),
			AssetType:    types.StringValue(asset.Node.Asset_type),
			ReferenceIDs: referenceIDs,
			Annotations:  annotations,
			Score: scoreModel{
				Grade: types.StringValue(asset.Node.Score.Grade),
				Value: types.Int64Value(asset.Node.Score.Value),
			},
		}
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func convertToAnnotationsModel(kv KeyValue) *annotationsModel {
	return &annotationsModel{
		Key:   kv.Key,
		Value: kv.Value,
	}
}
