// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var _ resource.Resource = (*AssetRoutingRuleResource)(nil)

func NewAssetRoutingRuleResource() resource.Resource {
	return &AssetRoutingRuleResource{}
}

type AssetRoutingRuleResource struct {
	client *ExtendedGqlClient
}

type AssetRoutingRuleResourceModel struct {
	OrgMrn         types.String                 `tfsdk:"org_mrn"`
	Mrn            types.String                 `tfsdk:"mrn"`
	TargetSpaceMrn types.String                 `tfsdk:"target_space_mrn"`
	Priority       types.Int64                  `tfsdk:"priority"`
	Conditions     []AssetRoutingConditionModel `tfsdk:"condition"`
}

func (r *AssetRoutingRuleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_asset_routing_rule"
}

func (r *AssetRoutingRuleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages an individual asset routing rule for a Mondoo organization. This is a **non-authoritative** resource — it manages a single rule without affecting other rules. Multiple rules can coexist and be managed independently, making it ideal for multi-team setups where each team manages their own routing rules.

~> **Warning:** Do not use this resource together with ` + "`mondoo_asset_routing_table`" + ` for the same organization. The table resource replaces all rules atomically, which will overwrite individually managed rules.`,

		Attributes: map[string]schema.Attribute{
			"org_mrn": schema.StringAttribute{
				MarkdownDescription: "The Mondoo Resource Name (MRN) of the organization.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mrn": schema.StringAttribute{
				MarkdownDescription: "The Mondoo Resource Name (MRN) of the routing rule.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"target_space_mrn": schema.StringAttribute{
				MarkdownDescription: "The MRN of the space where matching assets will be routed.",
				Required:            true,
			},
			"priority": schema.Int64Attribute{
				MarkdownDescription: "The priority of this rule. Lower values are evaluated first. Rules with the same priority are further sorted by specificity (number of conditions) and MRN.",
				Required:            true,
			},
		},
		Blocks: map[string]schema.Block{
			"condition": assetRoutingConditionSchemaBlock(),
		},
	}
}

func (r *AssetRoutingRuleResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *AssetRoutingRuleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data AssetRoutingRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := CreateAssetRoutingRuleInput{
		OrgMrn:         mondoov1.String(data.OrgMrn.ValueString()),
		TargetSpaceMrn: mondoov1.String(data.TargetSpaceMrn.ValueString()),
		Priority:       mondoov1.Int(data.Priority.ValueInt64()),
		Conditions:     conditionsFromModel(data.Conditions),
	}

	tflog.Debug(ctx, "creating asset routing rule", map[string]interface{}{
		"orgMrn":   data.OrgMrn.ValueString(),
		"priority": data.Priority.ValueInt64(),
	})

	result, err := r.client.CreateAssetRoutingRule(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create asset routing rule", err.Error())
		return
	}

	data.Mrn = types.StringValue(result.Mrn)
	data.TargetSpaceMrn = types.StringValue(result.TargetSpaceMrn)
	data.Priority = types.Int64Value(int64(result.Priority))
	data.Conditions = conditionsToModel(result.Conditions)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AssetRoutingRuleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data AssetRoutingRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ruleMrn := data.Mrn.ValueString()
	result, err := r.client.GetAssetRoutingRule(ctx, ruleMrn)
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read asset routing rule", err.Error())
		return
	}

	data.Mrn = types.StringValue(result.Mrn)
	data.TargetSpaceMrn = types.StringValue(result.TargetSpaceMrn)
	data.Priority = types.Int64Value(int64(result.Priority))
	data.Conditions = conditionsToModel(result.Conditions)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AssetRoutingRuleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data AssetRoutingRuleResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	input := UpdateAssetRoutingRuleInput{
		RuleMrn:        mondoov1.String(data.Mrn.ValueString()),
		TargetSpaceMrn: mondoov1.String(data.TargetSpaceMrn.ValueString()),
		Priority:       mondoov1.Int(data.Priority.ValueInt64()),
		Conditions:     conditionsFromModel(data.Conditions),
	}

	tflog.Debug(ctx, "updating asset routing rule", map[string]interface{}{
		"ruleMrn":  data.Mrn.ValueString(),
		"priority": data.Priority.ValueInt64(),
	})

	result, err := r.client.UpdateAssetRoutingRule(ctx, input)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update asset routing rule", err.Error())
		return
	}

	data.Mrn = types.StringValue(result.Mrn)
	data.TargetSpaceMrn = types.StringValue(result.TargetSpaceMrn)
	data.Priority = types.Int64Value(int64(result.Priority))
	data.Conditions = conditionsToModel(result.Conditions)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *AssetRoutingRuleResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data AssetRoutingRuleResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	ruleMrn := data.Mrn.ValueString()
	tflog.Debug(ctx, "deleting asset routing rule", map[string]interface{}{
		"ruleMrn": ruleMrn,
	})

	err := r.client.DeleteAssetRoutingRule(ctx, ruleMrn)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete asset routing rule", err.Error())
	}
}

func (r *AssetRoutingRuleResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	ruleMrn := req.ID

	result, err := r.client.GetAssetRoutingRule(ctx, ruleMrn)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import asset routing rule", err.Error())
		return
	}

	// Extract org MRN from the rule MRN.
	// Rule MRN format: //policy.api.mondoo.app/organizations/{orgId}/routing-rules/{ruleId}
	orgMrn, err := orgMrnFromRuleMrn(ruleMrn)
	if err != nil {
		resp.Diagnostics.AddError("Failed to parse rule MRN", err.Error())
		return
	}

	data := AssetRoutingRuleResourceModel{
		OrgMrn:         types.StringValue(orgMrn),
		Mrn:            types.StringValue(result.Mrn),
		TargetSpaceMrn: types.StringValue(result.TargetSpaceMrn),
		Priority:       types.Int64Value(int64(result.Priority)),
		Conditions:     conditionsToModel(result.Conditions),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// orgMrnFromRuleMrn extracts the organization MRN from a routing rule MRN.
// Rule MRN format: //policy.api.mondoo.app/organizations/{orgId}/routing-rules/{ruleId}
// Org MRN format: //captain.api.mondoo.app/organizations/{orgId}
func orgMrnFromRuleMrn(ruleMrn string) (string, error) {
	const prefix = "//policy.api.mondoo.app/organizations/"
	if !strings.HasPrefix(ruleMrn, prefix) {
		return "", fmt.Errorf("invalid rule MRN format: %s", ruleMrn)
	}

	rest := ruleMrn[len(prefix):]
	// rest should be "{orgId}/routing-rules/{ruleId}"
	for i, ch := range rest {
		if ch == '/' {
			orgID := rest[:i]
			return orgPrefix + orgID, nil
		}
	}
	return "", fmt.Errorf("invalid rule MRN format: %s", ruleMrn)
}
