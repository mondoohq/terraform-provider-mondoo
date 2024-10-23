// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSpaceFrom(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Space
	}{
		{
			name:     "Valid MRN",
			input:    "//captain.api.mondoo.app/spaces/1234",
			expected: Space("1234"),
		},
		{
			name:     "Valid MRN with additional segments",
			input:    "//captain.api.mondoo.app/spaces/1234/resources/5678",
			expected: Space("1234"),
		},
		{
			name:     "Space ID only",
			input:    "5678",
			expected: Space("5678"),
		},
		{
			name:     "Empty string input",
			input:    "",
			expected: Space(""),
		},
		{
			name:     "Invalid MRN without space ID segment",
			input:    "//captain.api.mondoo.app/spaces/",
			expected: Space(""),
		},
		{
			name:     "Non-MRN format",
			input:    "not-an-mrn",
			expected: Space("not-an-mrn"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SpaceFrom(tt.input)
			assert.Equal(t, tt.expected, result, "Expected SpaceFrom result to match")
		})
	}
}

func TestSpace_ID(t *testing.T) {
	tests := []struct {
		name     string
		input    Space
		expected string
	}{
		{
			name:     "Valid Space ID",
			input:    Space("1234"),
			expected: "1234",
		},
		{
			name:     "Empty Space",
			input:    Space(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.ID()
			assert.Equal(t, tt.expected, result, "Expected ID result to match")
		})
	}
}

func TestSpace_MRN(t *testing.T) {
	tests := []struct {
		name     string
		input    Space
		expected string
	}{
		{
			name:     "Valid Space ID",
			input:    Space("1234"),
			expected: "//captain.api.mondoo.app/spaces/1234",
		},
		{
			name:     "Empty Space",
			input:    Space(""),
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.input.MRN()
			assert.Equal(t, tt.expected, result, "Expected MRN result to match")
		})
	}
}
