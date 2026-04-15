// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidIntegrationMrn(t *testing.T) {
	tests := []struct {
		name     string
		mrn      string
		expected bool
	}{
		{
			name:     "valid space-scoped integration MRN",
			mrn:      "//integration.api.mondoo.app/spaces/my-space-123/integrations/abc-def-456",
			expected: true,
		},
		{
			name:     "valid org-scoped integration MRN",
			mrn:      "//integration.api.mondoo.app/organizations/my-org-789/integrations/abc-def-456",
			expected: true,
		},
		{
			name:     "valid platform-scoped integration MRN",
			mrn:      "//integration.api.mondoo.app/integrations/abc-def-456",
			expected: true,
		},
		{
			name:     "invalid: wrong domain",
			mrn:      "//captain.api.mondoo.app/spaces/my-space/integrations/abc",
			expected: false,
		},
		{
			name:     "invalid: missing integrations segment",
			mrn:      "//integration.api.mondoo.app/spaces/my-space",
			expected: false,
		},
		{
			name:     "invalid: empty string",
			mrn:      "",
			expected: false,
		},
		{
			name:     "invalid: random string",
			mrn:      "not-an-mrn",
			expected: false,
		},
		{
			name:     "invalid: trailing slash",
			mrn:      "//integration.api.mondoo.app/spaces/my-space/integrations/abc/",
			expected: false,
		},
		{
			name:     "invalid: unknown scope type",
			mrn:      "//integration.api.mondoo.app/workspaces/ws-123/integrations/abc",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsValidIntegrationMrn(tt.mrn))
		})
	}
}

func TestIsValidPlatformIntegrationMrn(t *testing.T) {
	tests := []struct {
		name     string
		mrn      string
		expected bool
	}{
		{
			name:     "valid platform integration MRN",
			mrn:      "//integration.api.mondoo.app/integrations/abc-def-456",
			expected: true,
		},
		{
			name:     "invalid: space-scoped is not platform",
			mrn:      "//integration.api.mondoo.app/spaces/my-space/integrations/abc",
			expected: false,
		},
		{
			name:     "invalid: org-scoped is not platform",
			mrn:      "//integration.api.mondoo.app/organizations/my-org/integrations/abc",
			expected: false,
		},
		{
			name:     "invalid: empty string",
			mrn:      "",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, IsValidPlatformIntegrationMrn(tt.mrn))
		})
	}
}

func TestIntegration_ScopeMRN(t *testing.T) {
	tests := []struct {
		name     string
		mrn      string
		expected string
	}{
		{
			name:     "space-scoped integration",
			mrn:      "//integration.api.mondoo.app/spaces/my-space-123/integrations/int-456",
			expected: "//captain.api.mondoo.app/spaces/my-space-123",
		},
		{
			name:     "org-scoped integration",
			mrn:      "//integration.api.mondoo.app/organizations/my-org-789/integrations/int-456",
			expected: "//captain.api.mondoo.app/organizations/my-org-789",
		},
		{
			name:     "platform-scoped integration",
			mrn:      "//integration.api.mondoo.app/integrations/int-456",
			expected: "//platform.api.mondoo.app",
		},
		{
			name:     "invalid MRN returns empty",
			mrn:      "not-an-mrn",
			expected: "",
		},
		{
			name:     "empty MRN returns empty",
			mrn:      "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration := Integration{Mrn: tt.mrn}
			assert.Equal(t, tt.expected, integration.ScopeMRN())
		})
	}
}

func TestIntegration_IsSpaceScoped(t *testing.T) {
	tests := []struct {
		name     string
		mrn      string
		expected bool
	}{
		{
			name:     "space-scoped",
			mrn:      "//integration.api.mondoo.app/spaces/my-space/integrations/abc",
			expected: true,
		},
		{
			name:     "org-scoped",
			mrn:      "//integration.api.mondoo.app/organizations/my-org/integrations/abc",
			expected: false,
		},
		{
			name:     "platform-scoped",
			mrn:      "//integration.api.mondoo.app/integrations/abc",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integration := Integration{Mrn: tt.mrn}
			assert.Equal(t, tt.expected, integration.IsSpaceScoped())
		})
	}
}
