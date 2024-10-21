// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	mondoov1 "go.mondoo.com/mondoo-go"
)

func TestConvertListValue(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected types.List
	}{
		{
			name:     "Empty list",
			input:    []string{},
			expected: types.ListValueMust(types.StringType, []attr.Value(nil)),
		},
		{
			name:  "Single item list",
			input: []string{"hello"},
			expected: types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue("hello"),
			}),
		},
		{
			name:  "Multiple items list",
			input: []string{"hello", "world"},
			expected: types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue("hello"),
				types.StringValue("world"),
			}),
		},
		{
			name:  "List with special characters",
			input: []string{"hello", "world!", "@special#"},
			expected: types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue("hello"),
				types.StringValue("world!"),
				types.StringValue("@special#"),
			}),
		},
		{
			name:  "List with empty strings",
			input: []string{"", "hello", ""},
			expected: types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue(""),
				types.StringValue("hello"),
				types.StringValue(""),
			}),
		},
		{
			name:  "List with duplicate strings",
			input: []string{"duplicate", "duplicate"},
			expected: types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue("duplicate"),
				types.StringValue("duplicate"),
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertListValue(tt.input)
			assert.Equal(t, tt.expected, result,
				"Converted list should match expected output",
			)
		})
	}
}

func TestConvertSliceStrings(t *testing.T) {
	tests := []struct {
		name        string
		input       types.List
		expected    []mondoov1.String
		expectError bool
	}{
		{
			name:     "Empty list",
			input:    types.ListValueMust(types.StringType, []attr.Value{}),
			expected: []mondoov1.String{},
		},
		{
			name: "Single item list",
			input: types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue("hello"),
			}),
			expected: []mondoov1.String{"hello"},
		},
		{
			name: "Multiple items list",
			input: types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue("hello"),
				types.StringValue("world"),
			}),
			expected: []mondoov1.String{"hello", "world"},
		},
		{
			name: "List with special characters",
			input: types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue("hello"),
				types.StringValue("world!"),
				types.StringValue("@special#"),
			}),
			expected: []mondoov1.String{"hello", "world!", "@special#"},
		},
		{
			name: "List with empty strings",
			input: types.ListValueMust(types.StringType, []attr.Value{
				types.StringValue(""),
				types.StringValue("hello"),
				types.StringValue(""),
			}),
			expected: []mondoov1.String{"", "hello", ""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConvertSliceStrings(tt.input)
			assert.Equal(t, tt.expected, result,
				"Converted slice should match expected output",
			)
		})
	}
}
