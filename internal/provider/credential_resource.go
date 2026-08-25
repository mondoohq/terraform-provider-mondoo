// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"github.com/hashicorp/terraform-plugin-framework/types"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// credentialOptionalString maps an unset optional attribute to a nil pointer so
// the field is omitted from the GraphQL input entirely.
func credentialOptionalString(v types.String) *mondoov1.String {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return mondoov1.NewStringPtr(mondoov1.String(v.ValueString()))
}
