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

var _ datasource.DataSource = (*policyDataSource)(nil)

func NewPolicyDataSource() datasource.DataSource {
	return &policyDataSource{}
}

type policyDataSource struct {
	client *ExtendedGqlClient
}

type policyDataSourceModel struct {
	SpaceID      types.String  `tfsdk:"space_id"`
	SpaceMrn     types.String  `tfsdk:"space_mrn"`
	CatalogType  types.String  `tfsdk:"catalog_type"`
	AssignedOnly types.Bool    `tfsdk:"assigned_only"`
	Policies     []policyModel `tfsdk:"policies"`
}

func (d *policyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (d *policyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source for policies and querypacks",
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Space ID",
				Validators: []validator.String{
					// Validate only this attribute or other_attr is configured.
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
					// Validate only this attribute or other_attr is configured.
					stringvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("space_id"),
					}...),
				},
			},
			"catalog_type": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Catalog type of either `ALL`, `POLICY` or `QUERYPACK`. Defaults to `ALL`",
			},
			"assigned_only": schema.BoolAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Only return enabled policies if set to `true`",
			},
			"policies": schema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "List of policies",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"policy_mrn": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Unique policy Mondoo Resource Name",
						},
						"policy_name": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Policy name",
						},
						"assigned": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Determines if a policy is enabled or disabled",
						},
						"action": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Policies can be set to `Null`, `IGNORE` or `ACTIVE`",
						},
						"version": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Version",
						},
						"is_public": schema.BoolAttribute{
							Computed:            true,
							MarkdownDescription: "Determines if a policy is public or private",
						},
						"created_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Timestamp of policy creation",
						},
						"updated_at": schema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Timestamp of policy update",
						},
					},
				},
			},
		},
	}
}

func (d *policyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *policyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data policyDataSourceModel

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
	policies, err := d.client.GetPolicies(ctx, scopeMrn, data.CatalogType.ValueString(), data.AssignedOnly.ValueBool())
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch policies", err.Error())
		return
	}

	// Convert policies to the model
	data.Policies = make([]policyModel, len(*policies))
	for i, policy := range *policies {
		data.Policies[i] = policyModel{
			PolicyMrn:  types.StringValue(string(policy.Mrn)),
			PolicyName: types.StringValue(string(policy.Name)),
			Assigned:   types.BoolValue(bool(policy.Assigned)),
			Action:     types.StringValue(string(policy.Action)),
			Version:    types.StringValue(string(policy.Version)),
			IsPublic:   types.BoolValue(bool(policy.IsPublic)),
			CreatedAt:  types.StringValue(string(policy.CreatedAt)),
			UpdatedAt:  types.StringValue(string(policy.UpdatedAt)),
		}
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
