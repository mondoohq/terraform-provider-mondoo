// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"go.mondoo.com/terraform-provider-mondoo/internal/customtypes"
)

// RoleListNormalizer is a plan modifier that normalizes role MRNs in a list.
// It ensures that both short role names (e.g. "editor") and full MRNs
// (e.g. "//iam.api.mondoo.app/roles/editor") are treated as equivalent.
// The comparison is order-independent, treating the list as an unordered set.
type RoleListNormalizer struct{}

// Description returns a human-readable description of the plan modifier.
func (m RoleListNormalizer) Description(_ context.Context) string {
	return "Normalizes role names to full MRNs to prevent drift when switching between short and full formats."
}

// MarkdownDescription returns a markdown description of the plan modifier.
func (m RoleListNormalizer) MarkdownDescription(_ context.Context) string {
	return "Normalizes role names to full MRNs to prevent drift when switching between short and full formats."
}

// PlanModifyList implements the plan modifier logic for list attributes.
func (m RoleListNormalizer) PlanModifyList(ctx context.Context, req planmodifier.ListRequest, resp *planmodifier.ListResponse) {
	// If the entire resource is being created, we don't need to modify anything
	// The normalization will happen in the Create method
	if req.State.Raw.IsNull() {
		return
	}

	// If the plan is null or unknown, don't modify it
	if req.PlanValue.IsNull() || req.PlanValue.IsUnknown() {
		return
	}

	// If config is null or unknown, don't modify
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	// Get the current config values
	var configRoles []string
	resp.Diagnostics.Append(req.ConfigValue.ElementsAs(ctx, &configRoles, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get the current state values
	var stateRoles []string
	resp.Diagnostics.Append(req.StateValue.ElementsAs(ctx, &stateRoles, false)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Normalize both lists and compare
	normalizedConfig := make([]string, len(configRoles))
	for i, role := range configRoles {
		normalizedConfig[i] = customtypes.NormalizeRoleMRN(role)
	}

	normalizedState := make([]string, len(stateRoles))
	for i, role := range stateRoles {
		normalizedState[i] = customtypes.NormalizeRoleMRN(role)
	}

	// If the normalized values are the same (order-independent), use the state value to prevent drift
	// This allows users to:
	// 1. Switch between "editor" and "//iam.api.mondoo.app/roles/editor" without drift
	// 2. Reorder roles without triggering an update
	if roleSetMatches(normalizedConfig, normalizedState) {
		resp.PlanValue = req.StateValue
	}

	// Otherwise, keep the config value as-is (don't set resp.PlanValue)
	// Terraform will use the config value, and we'll normalize it in Create/Update
}

func roleSetMatches(a, b []string) bool {
	aSet := make(map[string]bool)
	for _, item := range a {
		aSet[item] = true
	}

	bSet := make(map[string]bool)
	for _, item := range b {
		bSet[item] = true
	}

	// Compare the maps
	if len(aSet) != len(bSet) {
		return false
	}

	for item := range aSet {
		if !bSet[item] {
			return false
		}
	}

	return true
}

// RoleListNormalizerModifier returns a plan modifier that normalizes role MRNs.
func RoleListNormalizerModifier() planmodifier.List {
	return RoleListNormalizer{}
}
