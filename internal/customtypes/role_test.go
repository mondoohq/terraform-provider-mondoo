// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package customtypes

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/stretchr/testify/assert"
)

func TestRoleValue_NormalizedValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Short role name",
			input:    "editor",
			expected: "//iam.api.mondoo.app/roles/editor",
		},
		{
			name:     "Full MRN",
			input:    "//iam.api.mondoo.app/roles/editor",
			expected: "//iam.api.mondoo.app/roles/editor",
		},
		{
			name:     "Another short role name",
			input:    "viewer",
			expected: "//iam.api.mondoo.app/roles/viewer",
		},
		{
			name:     "Owner role",
			input:    "owner",
			expected: "//iam.api.mondoo.app/roles/owner",
		},
		{
			name:     "Policy manager role",
			input:    "policy-manager",
			expected: "//iam.api.mondoo.app/roles/policy-manager",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			roleValue := NewRoleValue(tt.input)
			result := roleValue.NormalizedValue()
			assert.Equal(t, tt.expected, result, "Normalized value should match expected")
		})
	}
}

func TestRoleValue_StringSemanticEquals(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		value1   string
		value2   string
		expected bool
	}{
		{
			name:     "Short name equals full MRN",
			value1:   "editor",
			value2:   "//iam.api.mondoo.app/roles/editor",
			expected: true,
		},
		{
			name:     "Full MRN equals short name",
			value1:   "//iam.api.mondoo.app/roles/viewer",
			value2:   "viewer",
			expected: true,
		},
		{
			name:     "Both short names equal",
			value1:   "owner",
			value2:   "owner",
			expected: true,
		},
		{
			name:     "Both full MRNs equal",
			value1:   "//iam.api.mondoo.app/roles/editor",
			value2:   "//iam.api.mondoo.app/roles/editor",
			expected: true,
		},
		{
			name:     "Different roles - short names",
			value1:   "editor",
			value2:   "viewer",
			expected: false,
		},
		{
			name:     "Different roles - mixed",
			value1:   "editor",
			value2:   "//iam.api.mondoo.app/roles/viewer",
			expected: false,
		},
		{
			name:     "Different roles - full MRNs",
			value1:   "//iam.api.mondoo.app/roles/editor",
			value2:   "//iam.api.mondoo.app/roles/owner",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role1 := NewRoleValue(tt.value1)
			role2 := NewRoleValue(tt.value2)

			result, diags := role1.StringSemanticEquals(ctx, role2)

			assert.False(t, diags.HasError(), "Should not have diagnostics errors")
			assert.Equal(t, tt.expected, result, "Semantic equality should match expected")
		})
	}
}

func TestRoleValue_Equal(t *testing.T) {
	tests := []struct {
		name     string
		value1   string
		value2   string
		expected bool
	}{
		{
			name:     "Same values",
			value1:   "editor",
			value2:   "editor",
			expected: true,
		},
		{
			name:     "Different values",
			value1:   "editor",
			value2:   "viewer",
			expected: false,
		},
		{
			name:     "Short name vs full MRN - not equal in Equal method",
			value1:   "editor",
			value2:   "//iam.api.mondoo.app/roles/editor",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			role1 := NewRoleValue(tt.value1)
			role2 := NewRoleValue(tt.value2)

			result := role1.Equal(role2)

			assert.Equal(t, tt.expected, result, "Equality should match expected")
		})
	}
}

func TestRoleValue_NullAndUnknown(t *testing.T) {
	t.Run("Null value returns empty normalized value", func(t *testing.T) {
		roleValue := NewRoleNull()
		assert.True(t, roleValue.IsNull())
		assert.Equal(t, "", roleValue.NormalizedValue())
	})

	t.Run("Unknown value returns empty normalized value", func(t *testing.T) {
		roleValue := NewRoleUnknown()
		assert.True(t, roleValue.IsUnknown())
		assert.Equal(t, "", roleValue.NormalizedValue())
	})

	t.Run("Null values are equal", func(t *testing.T) {
		role1 := NewRoleNull()
		role2 := NewRoleNull()
		assert.True(t, role1.Equal(role2))
	})
}

func TestRoleType_ValueFromString(t *testing.T) {
	ctx := context.Background()
	roleType := RoleType{}

	t.Run("Convert string value to RoleValue", func(t *testing.T) {
		stringValue := basetypes.NewStringValue("editor")

		roleValuable, diags := roleType.ValueFromString(ctx, stringValue)

		assert.False(t, diags.HasError(), "Should not have diagnostics errors")
		roleValue, ok := roleValuable.(RoleValue)
		assert.True(t, ok, "Should return RoleValue type")
		assert.Equal(t, "editor", roleValue.ValueString())
		assert.Equal(t, "//iam.api.mondoo.app/roles/editor", roleValue.NormalizedValue())
	})
}

func TestNormalizeRoleMRN(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Short role name",
			input:    "editor",
			expected: "//iam.api.mondoo.app/roles/editor",
		},
		{
			name:     "Full MRN already",
			input:    "//iam.api.mondoo.app/roles/editor",
			expected: "//iam.api.mondoo.app/roles/editor",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Role with dashes",
			input:    "policy-manager",
			expected: "//iam.api.mondoo.app/roles/policy-manager",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeRoleMRN(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
