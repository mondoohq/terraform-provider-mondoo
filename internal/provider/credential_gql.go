// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// The SDK generates input types and enums only, so every response struct below
// is declared here. The shapes were confirmed by introspecting the live schema:
//
//	credentialV2(mrn: ID!): CredentialV2
//	createCredentialV2(input: CreateCredentialV2Input!): CreateCredentialV2Payload!
//	rotateCredentialV2(input: RotateCredentialV2Input!): RotateCredentialV2Payload!
//	renameCredentialV2(input: RenameCredentialV2Input!): RenameCredentialV2Payload!
//	deleteCredentialV2(input: DeleteCredentialV2Input!): DeleteCredentialV2Payload!

// CredentialV2IntegrationUsage is the one member of the CredentialV2Usage
// union: an integration slot filled by a credential.
type CredentialV2IntegrationUsage struct {
	Mrn     string // integration MRN
	Name    string // display name, may be empty
	Purpose string // which slot: "default", or a named slot
	Type    string // ClientIntegrationType, for icon and label
}

// CredentialV2Usage is the usages union. It currently has a single possible
// type, so the inline fragment below is the whole selection set.
type CredentialV2Usage struct {
	IntegrationUsage CredentialV2IntegrationUsage `graphql:"... on CredentialV2IntegrationUsage"`
}

// CredentialV2FieldValue is one entry of the credential's redacted field map.
// The key is called `name`, not `key`.
type CredentialV2FieldValue struct {
	Name  string
	Value string
}

// CredentialV2 is the read side of a typed credential. The secret is never part
// of it — no read returns one — so FieldValues carries the non-secret fields
// only, from the row's redacted copy, plus the fields the server derived from
// the secret.
type CredentialV2 struct {
	Mrn             string
	OwnerMrn        string
	Name            string
	Kind            string
	HealthStatus    string
	HealthCheckedAt *string
	HealthError     *string
	ExpiresAt       *string
	CreatedBy       *string
	CreatedAt       string
	UpdatedAt       string
	FieldValues     []CredentialV2FieldValue
	// Usages is non-nullable on purpose: a failed read must be a GraphQL error,
	// never an empty list, because empty is the answer that makes deleting look
	// safe.
	Usages []CredentialV2Usage
}

// FieldValuesMap flattens FieldValues for the Terraform map attribute.
func (c CredentialV2) FieldValuesMap() map[string]string {
	out := make(map[string]string, len(c.FieldValues))
	for _, fv := range c.FieldValues {
		out[fv.Name] = fv.Value
	}
	return out
}

// GetCredentialV2 reads a credential. The query field is nullable, so a nil
// result means the credential no longer exists rather than being an error.
func (c *ExtendedGqlClient) GetCredentialV2(ctx context.Context, mrn string) (*CredentialV2, error) {
	var q struct {
		CredentialV2 *CredentialV2 `graphql:"credentialV2(mrn: $mrn)"`
	}
	variables := map[string]interface{}{
		"mrn": mondoov1.ID(mrn),
	}

	if err := c.Query(ctx, &q, variables); err != nil {
		return nil, err
	}
	return q.CredentialV2, nil
}

type createCredentialV2Payload struct {
	Credential CredentialV2
}

func (c *ExtendedGqlClient) CreateCredentialV2(
	ctx context.Context, scopeMrn, name string, secret mondoov1.CredentialV2SecretInput,
) (CredentialV2, error) {
	var m struct {
		CreateCredentialV2 createCredentialV2Payload `graphql:"createCredentialV2(input: $input)"`
	}
	input := mondoov1.CreateCredentialV2Input{
		ScopeMrn: mondoov1.ID(scopeMrn),
		Name:     mondoov1.String(name),
		Secret:   secret,
	}
	tflog.Trace(ctx, "CreateCredentialV2", map[string]interface{}{
		"scopeMrn": scopeMrn,
		"name":     name,
	})

	if err := c.Mutate(ctx, &m, input, nil); err != nil {
		return CredentialV2{}, err
	}
	return m.CreateCredentialV2.Credential, nil
}

type renameCredentialV2Payload struct {
	Credential CredentialV2
}

func (c *ExtendedGqlClient) RenameCredentialV2(ctx context.Context, mrn, name string) (CredentialV2, error) {
	var m struct {
		RenameCredentialV2 renameCredentialV2Payload `graphql:"renameCredentialV2(input: $input)"`
	}
	input := mondoov1.RenameCredentialV2Input{
		Mrn:  mondoov1.ID(mrn),
		Name: mondoov1.String(name),
	}
	tflog.Trace(ctx, "RenameCredentialV2", map[string]interface{}{"mrn": mrn, "name": name})

	if err := c.Mutate(ctx, &m, input, nil); err != nil {
		return CredentialV2{}, err
	}
	return m.RenameCredentialV2.Credential, nil
}

type rotateCredentialV2Payload struct {
	Credential CredentialV2
}

// RotateCredentialV2 replaces the whole secret payload. It does not merge, so
// the caller must send every field of the arm.
func (c *ExtendedGqlClient) RotateCredentialV2(
	ctx context.Context, mrn string, secret mondoov1.CredentialV2SecretInput,
) (CredentialV2, error) {
	var m struct {
		RotateCredentialV2 rotateCredentialV2Payload `graphql:"rotateCredentialV2(input: $input)"`
	}
	input := mondoov1.RotateCredentialV2Input{
		Mrn:    mondoov1.ID(mrn),
		Secret: secret,
	}
	tflog.Trace(ctx, "RotateCredentialV2", map[string]interface{}{"mrn": mrn})

	if err := c.Mutate(ctx, &m, input, nil); err != nil {
		return CredentialV2{}, err
	}
	return m.RotateCredentialV2.Credential, nil
}

type deleteCredentialV2Payload struct {
	DeletedMrn string
}

// DeleteCredentialV2 fails with FailedPrecondition while any integration
// references the credential. Orphaning is refused deliberately server-side.
func (c *ExtendedGqlClient) DeleteCredentialV2(ctx context.Context, mrn string) (string, error) {
	var m struct {
		DeleteCredentialV2 deleteCredentialV2Payload `graphql:"deleteCredentialV2(input: $input)"`
	}
	input := mondoov1.DeleteCredentialV2Input{
		Mrn: mondoov1.ID(mrn),
	}
	tflog.Trace(ctx, "DeleteCredentialV2", map[string]interface{}{"mrn": mrn})

	if err := c.Mutate(ctx, &m, input, nil); err != nil {
		return "", err
	}
	return m.DeleteCredentialV2.DeletedMrn, nil
}

// credentialUsageSummary renders usages for a diagnostic: one line per slot,
// naming the integration and which slot it fills.
func credentialUsageSummary(usages []CredentialV2Usage) string {
	if len(usages) == 0 {
		return ""
	}
	var out string
	for _, u := range usages {
		name := u.IntegrationUsage.Name
		if name == "" {
			name = u.IntegrationUsage.Mrn
		}
		out += fmt.Sprintf("\n  - %s (%s, slot %q)", name, u.IntegrationUsage.Mrn, u.IntegrationUsage.Purpose)
	}
	return out
}
