// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

// Package schemacheck holds offline checks over the provider schema. It lives
// outside internal/provider because that package's TestMain needs a Mondoo
// service account, and these checks must run without one.
package schemacheck

import "regexp"

// credentialName matches attribute names that carry secret material. It is
// anchored at the end so that identifiers such as service_account_email,
// credential_mrn or wif_auth_binding_mrn do not match.
var credentialName = regexp.MustCompile(`(^|_)(token|secret|password|passphrase|credentials?|certificate|pem_file|service_account(_json)?|(private|secret|access|api|service_account)_key)$`)

// IsCredentialName reports whether a snake_case attribute name looks like it
// holds secret material. The code generator marks matching attributes
// Sensitive, and TestCredentialAttributesAreSensitive checks every attribute
// against it, so the two cannot disagree.
func IsCredentialName(name string) bool {
	return credentialName.MatchString(name)
}
