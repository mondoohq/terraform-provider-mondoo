// Copyright Mondoo, Inc. 2024, 2026
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
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var _ resource.Resource = (*AssetRoutingTableResource)(nil)

func NewAssetRoutingTableResource() resource.Resource {
	return &AssetRoutingTableResource{}
}

type AssetRoutingTableResource struct {
	client *ExtendedGqlClient
}

type AssetRoutingTableResourceModel struct {
	OrgMrn types.String                 `tfsdk:"org_mrn"`
	Rules  []AssetRoutingTableRuleModel `tfsdk:"rule"`
}

type AssetRoutingTableRuleModel struct {
	TargetSpaceMrn types.String                 `tfsdk:"target_space_mrn"`
	Conditions     []AssetRoutingConditionModel `tfsdk:"condition"`
}

func (r *AssetRoutingTableResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset_routing_table"
}

func (r *AssetRoutingTableResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages the asset routing table for a Mondoo organization. This is an **authoritative** resource — it manages the entire routing table and replaces all rules on every apply. Priority is derived from the order of rules in the configuration (first rule = highest priority).

~> **Warning:** Do not use this resource together with ` + "`mondoo_asset_routing_rule`" + ` for the same organization. The table resource replaces all rules atomically, which will overwrite individually managed rules.`,

		Attributes: map[string]schema.Attribute{
			"org_mrn": schema.StringAttribute{
				MarkdownDescription: "The Mondoo Resource Name (MRN) of the organization.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"rule": schema.ListNestedBlock{
				MarkdownDescription: "Ordered list of routing rules. Priority is determined by position (first = highest priority). A rule with no conditions acts as a catch-all.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"target_space_mrn": schema.StringAttribute{
							MarkdownDescription: "The MRN of the space where matching assets will be routed.",
							Required:            true,
						},
					},
					Blocks: map[string]schema.Block{
						"condition": assetRoutingConditionSchemaBlock(),
					},
				},
			},
		},
	}
}

func (r *AssetRoutingTableResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AssetRoutingTableResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AssetRoutingTableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := tableInputFromModel(data)
	tflog.Debug(ctx, "creating asset routing table", map[string]interface{}{
		"orgMrn": data.OrgMrn.ValueString(),
		"rules":  len(data.Rules),
	})

	result, err := r.client.SetAssetRoutingTable(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create asset routing table", err.Error())
		return
	}

	data.Rules = tableRulesFromPayload(result.Rules)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AssetRoutingTableResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AssetRoutingTableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgMrn := data.OrgMrn.ValueString()
	result, err := r.client.GetAssetRoutingTable(ctx, orgMrn)
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read asset routing table", err.Error())
		return
	}

	data.OrgMrn = types.StringValue(result.OrgMrn)
	data.Rules = tableRulesFromPayload(result.Rules)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AssetRoutingTableResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AssetRoutingTableResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := tableInputFromModel(data)
	tflog.Debug(ctx, "updating asset routing table", map[string]interface{}{
		"orgMrn": data.OrgMrn.ValueString(),
		"rules":  len(data.Rules),
	})

	result, err := r.client.SetAssetRoutingTable(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update asset routing table", err.Error())
		return
	}

	data.Rules = tableRulesFromPayload(result.Rules)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AssetRoutingTableResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AssetRoutingTableResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgMrn := data.OrgMrn.ValueString()
	tflog.Debug(ctx, "clearing asset routing table", map[string]interface{}{
		"orgMrn": orgMrn,
	})

	err := r.client.ClearAssetRoutingTable(ctx, orgMrn)
	if err != nil {
		resp.Diagnostics.AddError("Failed to clear asset routing table", err.Error())
	}
}

func (r *AssetRoutingTableResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	orgMrn := req.ID

	result, err := r.client.GetAssetRoutingTable(ctx, orgMrn)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import asset routing table", err.Error())
		return
	}

	data := AssetRoutingTableResourceModel{
		OrgMrn: types.StringValue(result.OrgMrn),
		Rules:  tableRulesFromPayload(result.Rules),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// tableInputFromModel converts the Terraform model to a GraphQL SetAssetRoutingTableInput.
func tableInputFromModel(data AssetRoutingTableResourceModel) SetAssetRoutingTableInput {
	rules := make([]AssetRoutingRuleInput, len(data.Rules))
	for i, rule := range data.Rules {
		rules[i] = AssetRoutingRuleInput{
			TargetSpaceMrn: mondoov1.String(rule.TargetSpaceMrn.ValueString()),
			Conditions:     conditionsFromModel(rule.Conditions),
		}
	}
	return SetAssetRoutingTableInput{
		OrgMrn: mondoov1.String(data.OrgMrn.ValueString()),
		Rules:  rules,
	}
}

// tableRulesFromPayload converts GraphQL rule payloads to Terraform models.
// Rules are expected to come back in priority order from the API.
func tableRulesFromPayload(rules []AssetRoutingRulePayload) []AssetRoutingTableRuleModel {
	result := make([]AssetRoutingTableRuleModel, len(rules))
	for i, rule := range rules {
		result[i] = AssetRoutingTableRuleModel{
			TargetSpaceMrn: types.StringValue(rule.TargetSpaceMrn),
			Conditions:     conditionsToModel(rule.Conditions),
		}
	}
	return result
}
