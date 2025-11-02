// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package customtypes

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

const roleMRNPrefix = "//iam.api.mondoo.app/roles/"

// Ensure the implementation satisfies the expected interfaces
var (
	_ basetypes.StringTypable                    = RoleType{}
	_ basetypes.StringValuableWithSemanticEquals = RoleValue{}
	_ basetypes.ListTypable                      = RoleListType{}
	_ basetypes.ListValuableWithSemanticEquals   = RoleListValue{}
)

// RoleType represents a Mondoo IAM role that can be specified as either
// a short name (e.g. "editor") or a full MRN (e.g. "//iam.api.mondoo.app/roles/editor").
// These two forms are treated as semantically equal.
type RoleType struct {
	basetypes.StringType
}

func (t RoleType) Equal(o attr.Type) bool {
	other, ok := o.(RoleType)
	if !ok {
		return false
	}
	return t.StringType.Equal(other.StringType)
}

func (t RoleType) String() string {
	return "RoleType"
}

func (t RoleType) ValueFromString(
	ctx context.Context,
	in basetypes.StringValue,
) (basetypes.StringValuable, diag.Diagnostics) {
	value := RoleValue{
		StringValue: in,
	}
	return value, nil
}

func (t RoleType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.StringType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	stringValue, ok := attrValue.(basetypes.StringValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}

	stringValuable, diags := t.ValueFromString(ctx, stringValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting StringValue to StringValuable: %v", diags)
	}

	return stringValuable, nil
}

func (t RoleType) ValueType(ctx context.Context) attr.Value {
	return RoleValue{}
}

// RoleValue is a custom value type for Mondoo IAM roles.
type RoleValue struct {
	basetypes.StringValue
}

// func (v RoleValue) Equal(o attr.Value) bool {
// 	other, ok := o.(RoleValue)
// 	if !ok {
// 		return false
// 	}
// 	return v.StringValue.Equal(other.StringValue)
// }

func (v RoleValue) Type(ctx context.Context) attr.Type {
	return RoleType{}
}

// StringSemanticEquals compares two role values semantically.
// "editor" and "//iam.api.mondoo.app/roles/editor" are considered equal.
func (v RoleValue) StringSemanticEquals(
	ctx context.Context,
	newValuable basetypes.StringValuable,
) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	newValue, ok := newValuable.(RoleValue)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			"An unexpected value type was received while performing semantic equality checks.",
		)
		return false, diags
	}

	// Normalize both values and compare
	oldNormalized := normalizeRoleMRN(v.ValueString())
	newNormalized := normalizeRoleMRN(newValue.ValueString())

	return oldNormalized == newNormalized, diags
}

// NormalizedValue returns the role value with the full MRN prefix.
// This should be used when sending values to the API.
func (v RoleValue) NormalizedValue() string {
	if v.IsNull() || v.IsUnknown() {
		return ""
	}
	return normalizeRoleMRN(v.ValueString())
}

// NormalizeRoleMRN ensures the role has the full MRN prefix.
// This is exported so it can be used by the provider resources.
func NormalizeRoleMRN(role string) string {
	if role == "" {
		return ""
	}
	if strings.HasPrefix(role, roleMRNPrefix) {
		return role
	}
	return roleMRNPrefix + role
}

// normalizeRoleMRN is an internal alias for backwards compatibility
func normalizeRoleMRN(role string) string {
	return NormalizeRoleMRN(role)
}

// NewRoleValue creates a new RoleValue from a string.
func NewRoleValue(value string) RoleValue {
	return RoleValue{
		StringValue: basetypes.NewStringValue(value),
	}
}

// NewRoleNull creates a new null RoleValue.
func NewRoleNull() RoleValue {
	return RoleValue{
		StringValue: basetypes.NewStringNull(),
	}
}

// NewRoleUnknown creates a new unknown RoleValue.
func NewRoleUnknown() RoleValue {
	return RoleValue{
		StringValue: basetypes.NewStringUnknown(),
	}
}

// RoleListType is a custom list type for Mondoo IAM roles with semantic equality.
type RoleListType struct {
	basetypes.ListType
}

func (t RoleListType) Equal(o attr.Type) bool {
	other, ok := o.(RoleListType)
	if !ok {
		return false
	}
	return t.ListType.Equal(other.ListType)
}

func (t RoleListType) String() string {
	return "RoleListType"
}

func (t RoleListType) ValueFromList(ctx context.Context, in basetypes.ListValue) (basetypes.ListValuable, diag.Diagnostics) {
	value := RoleListValue{
		ListValue: in,
	}
	return value, nil
}

func (t RoleListType) ValueFromTerraform(ctx context.Context, in tftypes.Value) (attr.Value, error) {
	attrValue, err := t.ListType.ValueFromTerraform(ctx, in)
	if err != nil {
		return nil, err
	}

	listValue, ok := attrValue.(basetypes.ListValue)
	if !ok {
		return nil, fmt.Errorf("unexpected value type of %T", attrValue)
	}

	listValuable, diags := t.ValueFromList(ctx, listValue)
	if diags.HasError() {
		return nil, fmt.Errorf("unexpected error converting ListValue to ListValuable: %v", diags)
	}

	return listValuable, nil
}

func (t RoleListType) ValueType(ctx context.Context) attr.Value {
	return RoleListValue{}
}

// RoleListValue is a custom list value type for Mondoo IAM roles.
type RoleListValue struct {
	basetypes.ListValue
}

func (v RoleListValue) Type(ctx context.Context) attr.Type {
	return RoleListType{
		ListType: basetypes.ListType{
			ElemType: RoleType{},
		},
	}
}

func (v RoleListValue) Equal(o attr.Value) bool {
	other, ok := o.(RoleListValue)
	if !ok {
		return false
	}
	return v.ListValue.Equal(other.ListValue)
}

// ListSemanticEquals compares two role lists semantically.
// Each role in the list is compared using normalized values.
func (v RoleListValue) ListSemanticEquals(ctx context.Context, newValuable basetypes.ListValuable) (bool, diag.Diagnostics) {
	var diags diag.Diagnostics

	newValue, ok := newValuable.(RoleListValue)
	if !ok {
		diags.AddError(
			"Semantic Equality Check Error",
			"An unexpected value type was received while performing semantic equality checks.",
		)
		return false, diags
	}

	// Get the elements from both lists
	oldElements := v.Elements()
	newElements := newValue.Elements()

	// Lists must have the same length
	if len(oldElements) != len(newElements) {
		return false, diags
	}

	// Compare each element semantically
	for i := 0; i < len(oldElements); i++ {
		oldRole, ok := oldElements[i].(RoleValue)
		if !ok {
			// Try to convert from StringValue
			if oldStr, ok := oldElements[i].(basetypes.StringValue); ok {
				oldRole = RoleValue{StringValue: oldStr}
			} else {
				diags.AddError(
					"Semantic Equality Check Error",
					fmt.Sprintf("Element at index %d is not a RoleValue", i),
				)
				return false, diags
			}
		}

		newRole, ok := newElements[i].(RoleValue)
		if !ok {
			// Try to convert from StringValue
			if newStr, ok := newElements[i].(basetypes.StringValue); ok {
				newRole = RoleValue{StringValue: newStr}
			} else {
				diags.AddError(
					"Semantic Equality Check Error",
					fmt.Sprintf("Element at index %d is not a RoleValue", i),
				)
				return false, diags
			}
		}

		// Compare normalized values
		if normalizeRoleMRN(oldRole.ValueString()) != normalizeRoleMRN(newRole.ValueString()) {
			return false, diags
		}
	}

	return true, diags
}

// NewRoleListValue creates a new RoleListValue from a list of role strings.
func NewRoleListValue(roles []string) RoleListValue {
	elements := make([]attr.Value, len(roles))
	for i, role := range roles {
		elements[i] = NewRoleValue(role)
	}

	listValue, _ := basetypes.NewListValue(RoleType{}, elements)
	return RoleListValue{
		ListValue: listValue,
	}
}

// NewRoleListNull creates a new null RoleListValue.
func NewRoleListNull() RoleListValue {
	return RoleListValue{
		ListValue: basetypes.NewListNull(RoleType{}),
	}
}

// NewRoleListUnknown creates a new unknown RoleListValue.
func NewRoleListUnknown() RoleListValue {
	return RoleListValue{
		ListValue: basetypes.NewListUnknown(RoleType{}),
	}
}

// NewRoleListValueFromList creates a RoleListValue from a basetypes.ListValue.
// This is useful when converting API responses.
func NewRoleListValueFromList(ctx context.Context, listValue basetypes.ListValue) (RoleListValue, error) {
	// Extract string values from the list
	var stringValues []string
	diags := listValue.ElementsAs(ctx, &stringValues, false)
	if diags.HasError() {
		return RoleListValue{}, fmt.Errorf("failed to extract elements: %v", diags)
	}

	// Convert to RoleValue elements
	elements := make([]attr.Value, len(stringValues))
	for i, str := range stringValues {
		elements[i] = NewRoleValue(str)
	}

	// Create the list
	newList, diag := basetypes.NewListValue(RoleType{}, elements)
	if diag.HasError() {
		return RoleListValue{}, fmt.Errorf("failed to create list: %v", diag)
	}

	return RoleListValue{ListValue: newList}, nil
}
