// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// AssetRoutingConditionModel is the Terraform model for a single routing condition.
// Shared between mondoo_asset_routing_table and mondoo_asset_routing_rule.
type AssetRoutingConditionModel struct {
	Field    types.String `tfsdk:"field"`
	Operator types.String `tfsdk:"operator"`
	Values   types.List   `tfsdk:"values"`
	Key      types.String `tfsdk:"key"`
}

// assetRoutingConditionSchemaAttributes returns the schema attributes for a routing condition,
// reusable by both the table and rule resources.
func assetRoutingConditionSchemaAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"field": schema.StringAttribute{
			MarkdownDescription: "The field to match on. Valid values: `HOSTNAME`, `PLATFORM`, `LABEL`.",
			Required:            true,
		},
		"operator": schema.StringAttribute{
			MarkdownDescription: "The comparison operator. Valid values: `EQUAL`, `NOT_EQUAL`, `CONTAINS`, `MATCHES`.",
			Required:            true,
		},
		"values": schema.ListAttribute{
			MarkdownDescription: "List of values to match against. A condition matches if the field matches any of the listed values (OR logic).",
			Required:            true,
			ElementType:         types.StringType,
		},
		"key": schema.StringAttribute{
			MarkdownDescription: "The label key to match on. Required when `field` is `LABEL`.",
			Optional:            true,
		},
	}
}

// conditionsFromModel converts Terraform condition models to GraphQL input types.
func conditionsFromModel(conditions []AssetRoutingConditionModel) []AssetRoutingConditionInput {
	result := make([]AssetRoutingConditionInput, len(conditions))
	for i, c := range conditions {
		values := make([]mondoov1.String, 0)
		if !c.Values.IsNull() && !c.Values.IsUnknown() {
			for _, v := range c.Values.Elements() {
				if sv, ok := v.(types.String); ok {
					values = append(values, mondoov1.String(sv.ValueString()))
				}
			}
		}

		input := AssetRoutingConditionInput{
			Field:    AssetRoutingConditionField(c.Field.ValueString()),
			Operator: AssetRoutingConditionOperator(c.Operator.ValueString()),
			Values:   values,
		}
		if !c.Key.IsNull() && !c.Key.IsUnknown() && c.Key.ValueString() != "" {
			key := mondoov1.String(c.Key.ValueString())
			input.Key = &key
		}
		result[i] = input
	}
	return result
}

// conditionsToModel converts GraphQL condition payloads to Terraform models.
func conditionsToModel(conditions []AssetRoutingConditionPayload) []AssetRoutingConditionModel {
	result := make([]AssetRoutingConditionModel, len(conditions))
	for i, c := range conditions {
		values := make([]string, len(c.Values))
		copy(values, c.Values)

		model := AssetRoutingConditionModel{
			Field:    types.StringValue(c.Field),
			Operator: types.StringValue(c.Operator),
			Values:   ConvertListValue(values),
		}
		if c.Key != "" {
			model.Key = types.StringValue(c.Key)
		} else {
			model.Key = types.StringNull()
		}
		result[i] = model
	}
	return result
}
