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

func (v RoleValue) Equal(o attr.Value) bool {
	other, ok := o.(RoleValue)
	if !ok {
		return false
	}
	return v.StringValue.Equal(other.StringValue)
}

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
