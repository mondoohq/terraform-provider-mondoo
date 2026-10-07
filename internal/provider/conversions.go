// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// ConvertListValue converts a slice of strings to a types.List.
func ConvertListValue[S ~string](list []S) types.List {
	var valueList []attr.Value
	for _, str := range list {
		valueList = append(valueList, types.StringValue(string(str)))
	}
	// Ensure the list is of type types.StringType
	return types.ListValueMust(types.StringType, valueList)
}

// ConvertSliceStrings converts a types.List to a slice of strings.
func ConvertSliceStrings(list types.List) (slice []mondoov1.String) {
	ctx := context.Background()
	allowlist, _ := list.ToListValue(ctx)
	allowlist.ElementsAs(ctx, &slice, true)
	return
}

// ConvertListValueInt32 converts a slice of int32 to a types.List.
func ConvertListValueInt32(list []int32) types.List {
	var valueList []attr.Value
	for _, val := range list {
		valueList = append(valueList, types.Int32Value(val))
	}
	// Ensure the list is of type types.Int32Type
	return types.ListValueMust(types.Int32Type, valueList)
}

// ConvertSliceInt32 converts a types.List to a slice of int32.
func ConvertSliceInt32(list types.List) (slice []mondoov1.Int) {
	ctx := context.Background()
	allowlist, _ := list.ToListValue(ctx)
	allowlist.ElementsAs(ctx, &slice, true)
	return
}

// ConvertSlice converts a types.List to a slice of strings.
func ConvertSlice[T any](list types.List) (slice []T) {
	ctx := context.Background()
	allowlist, _ := list.ToListValue(ctx)
	allowlist.ElementsAs(ctx, &slice, true)
	return
}

// ToPtr returns a pointer to the given value.
func ToPtr[T any](v T) *T {
	return &v
}

// OptionalStringPtr returns nil for a null or unknown string, so the field is
// left out of the request, and a pointer to the value otherwise. Use it for
// write-only secrets and credential references: the API reads an empty string
// as "set", so sending one alongside the alternative (an inline secret next to
// credentialMrn) fails as mutually exclusive, and on update it would overwrite
// what is stored.
func OptionalStringPtr(v types.String) *mondoov1.String {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return mondoov1.NewStringPtr(mondoov1.String(v.ValueString()))
}
