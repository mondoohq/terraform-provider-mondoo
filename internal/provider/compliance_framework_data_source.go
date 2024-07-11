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

var _ datasource.DataSource = (*complianceFrameworkDataSource)(nil)

func NewComplianceFrameworkDataSource() datasource.DataSource {
	return &complianceFrameworkDataSource{}
}

type complianceFrameworkDataSource struct {
	client *ExtendedGqlClient
}

type complianceFrameworkDataSourceModel struct {
	SpaceID              types.String               `tfsdk:"space_id"`
	SpaceMrn             types.String               `tfsdk:"space_mrn"`
	ComplianceFrameworks []complianceFrameworkModel `tfsdk:"compliance_frameworks"`
}

type author struct {
	Name  types.String `tfsdk:"name"`
	Email types.String `tfsdk:"email"`
}

type entry struct {
	Score     types.Float64 `tfsdk:"score"`
	Timestamp types.String  `tfsdk:"timestamp"`
}

type tag struct {
	Key   types.String `tfsdk:"key"`
	Value types.String `tfsdk:"value"`
}

type complianceFrameworkModel struct {
	Mrn                      types.String `tfsdk:"mrn"`
	Name                     types.String `tfsdk:"name"`
	State                    types.String `tfsdk:"state"`
	Authors                  []author     `tfsdk:"authors"`
	Tags                     []tag        `tfsdk:"tags"`
	PreviousCompletionScores []entry      `tfsdk:"previous_completion_scores"`
}

func (d *complianceFrameworkDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_compliance_framework"
}

func (d *complianceFrameworkDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source to return compliance frameworks in a Space",
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Space ID",
				Validators: []validator.String{
					// Validate only this attribute or space_mrn is configured.
					stringvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("space_mrn"),
					}...),
				},
			},
			"space_mrn": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Space MRN",
				Validators: []validator.String{
					// Validate only this attribute or space_id is configured.
					stringvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("space_id"),
					}...),
				},
			},
			"compliance_frameworks": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of compliance frameworks",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"mrn": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Compliance framework MRN",
						},
						"name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Compliance framework name",
						},
						"state": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Compliance framework state is either `PREVIEW` or `ACTIVE`",
						},
						"authors": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "List of authors",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"name": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Author name",
									},
									"email": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Author email",
									},
								},
							},
						},
						"tags": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "List of tags",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"key": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Tag key",
									},
									"value": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Tag value",
									},
								},
							},
						},
						"previous_completion_scores": schema.ListNestedAttribute{
							Computed:            true,
							MarkdownDescription: "List of previous completion scores",
							NestedObject: schema.NestedAttributeObject{
								Attributes: map[string]schema.Attribute{
									"score": schema.Float64Attribute{
										Computed:            true,
										MarkdownDescription: "Score",
									},
									"timestamp": schema.StringAttribute{
										Computed:            true,
										MarkdownDescription: "Timestamp",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (d *complianceFrameworkDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *complianceFrameworkDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data complianceFrameworkDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// generate space mrn
	scopeMrn := ""
	if data.SpaceMrn.ValueString() != "" {
		scopeMrn = data.SpaceMrn.ValueString()
	} else if data.SpaceID.ValueString() != "" {
		scopeMrn = spacePrefix + data.SpaceID.ValueString()
	}

	if scopeMrn == "" {
		resp.Diagnostics.AddError("Invalid Configuration", "Either `id` or `mrn` must be set")
		return
	}

	// Fetch policies
	frameworks, err := d.client.GetComplianceFrameworks(ctx, scopeMrn)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch frameworks", err.Error())
		return
	}

	// Make API request to fetch compliance frameworks
	data.ComplianceFrameworks = make([]complianceFrameworkModel, len(frameworks))
	for i, framework := range frameworks {

		authors := make([]author, len(framework.Authors))
		for j, a := range framework.Authors {
			authors[j] = author{
				Name:  types.StringValue(string(a.Name)),
				Email: types.StringValue(string(a.Email)),
			}
		}

		tags := make([]tag, len(framework.Tags))
		for j, t := range framework.Tags {
			tags[j] = tag{
				Key:   types.StringValue(string(t.Key)),
				Value: types.StringValue(string(t.Value)),
			}
		}

		entries := make([]entry, len(framework.PreviousCompletionScores.Entries))
		for j, e := range framework.PreviousCompletionScores.Entries {
			entries[j] = entry{
				Score:     types.Float64Value(float64(e.Score)),
				Timestamp: types.StringValue(string(e.Timestamp)),
			}
		}

		data.ComplianceFrameworks[i] = complianceFrameworkModel{
			Mrn:                      types.StringValue(string(framework.Mrn)),
			Name:                     types.StringValue(string(framework.Name)),
			State:                    types.StringValue(string(framework.State)),
			Authors:                  authors,
			Tags:                     tags,
			PreviousCompletionScores: entries,
		}
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
