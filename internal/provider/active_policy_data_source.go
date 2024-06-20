package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var _ datasource.DataSource = (*activePolicyDataSource)(nil)

func NewActivePolicyDataSource() datasource.DataSource {
	return &activePolicyDataSource{}
}

type activePolicyDataSource struct {
	client *ExtendedGqlClient
}

type policyModel struct {
	PolicyMrn  types.String `tfsdk:"policy_mrn"`
	PolicyName types.String `tfsdk:"policy_name"`
	Assigned   types.Bool   `tfsdk:"assigned"`
	Action     types.String `tfsdk:"action"`
	Version    types.String `tfsdk:"version"`
	IsPublic   types.Bool   `tfsdk:"is_public"`
	CreatedAt  types.String `tfsdk:"created_at"`
	UpdatedAt  types.String `tfsdk:"updated_at"`
}

type activePolicyDataSourceModel struct {
	SpaceID  types.String  `tfsdk:"space_id"`
	SpaceMrn types.String  `tfsdk:"space_mrn"`
	Policies []policyModel `tfsdk:"policies"`
}

func (d *activePolicyDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_active_policy"
}

func (d *activePolicyDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Data source that reports all active policies in a space",
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Space ID",
			},
			"space_mrn": schema.StringAttribute{
				Computed:            true,
				Optional:            true,
				MarkdownDescription: "Space MRN",
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
							MarkdownDescription: "Version of the policy",
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

func (d *activePolicyDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *activePolicyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data activePolicyDataSourceModel

	// Read Terraform configuration data into the model
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// generate space mrn
	spaceMrn := ""
	if data.SpaceMrn.ValueString() != "" && data.SpaceID.ValueString() == "" {
		spaceMrn = data.SpaceMrn.ValueString()
	} else if data.SpaceID.ValueString() != "" && data.SpaceMrn.ValueString() == "" {
		spaceMrn = spacePrefix + data.SpaceID.ValueString()
	} else {
		resp.Diagnostics.AddError("Invalid Configuration", "Either `id` or `mrn` must be set")
		return
	}

	// Fetch data from the API
	policies, err := d.client.GetPolicySpaceReport(ctx, spaceMrn)
	if err != nil {
		resp.Diagnostics.AddError("Failed to fetch policy", err.Error())
		return
	}

	// Extract policy data and populate the list of policies
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
