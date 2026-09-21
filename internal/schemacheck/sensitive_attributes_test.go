// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: BUSL-1.1

// Package schemacheck holds offline checks over the provider schema. It lives
// outside internal/provider because that package's TestMain needs a Mondoo
// service account, and these checks must run without one.
package schemacheck

import (
	"context"
	"regexp"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"go.mondoo.com/terraform-provider-mondoo/internal/provider"
)

// credentialAttributeName matches attribute names that carry secret material.
// It is anchored at the end so that identifiers such as service_account_email
// or wif_auth_binding_mrn do not match.
var credentialAttributeName = regexp.MustCompile(`(^|_)(token|secret|password|passphrase|credentials?|certificate|pem_file|service_account(_json)?|(private|secret|access|api|service_account)_key)$`)

// notSecret lists attributes whose name matches credentialAttributeName but
// which hold no secret material. Keys are "<schema>.<attribute path>"; values
// say why the attribute is safe to render.
var notSecret = map[string]string{}

// TestCredentialAttributesAreSensitive fails when an attribute that looks like
// a credential is not marked Sensitive. Terraform renders non-sensitive values
// verbatim in plan and apply output, so a missing flag leaks the secret into
// CI logs and pull request comments.
func TestCredentialAttributesAreSensitive(t *testing.T) {
	ctx := context.Background()
	server, err := providerserver.NewProtocol6WithError(provider.New("test")())()
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.GetProviderSchema(ctx, &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		t.Fatalf("provider schema: %s: %s", d.Summary, d.Detail)
	}

	var missing []string
	check := func(schemaName string, s *tfprotov6.Schema) {
		if s == nil || s.Block == nil {
			return
		}
		missing = append(missing, unmarkedCredentials(schemaName, s.Block.Attributes, false)...)
	}

	check("provider", resp.Provider)
	for name, s := range resp.ResourceSchemas {
		check(name, s)
	}
	for name, s := range resp.DataSourceSchemas {
		check("data."+name, s)
	}
	if len(resp.ResourceSchemas) == 0 {
		t.Fatal("provider returned no resource schemas; the check did not run")
	}

	sort.Strings(missing)
	for _, path := range missing {
		t.Errorf("%s looks like a credential but is not marked Sensitive; set Sensitive: true, or add it to notSecret with a reason", path)
	}
}

// unmarkedCredentials returns the paths of credential-named leaf attributes
// that are neither Sensitive themselves nor nested under a Sensitive parent.
func unmarkedCredentials(path string, attrs []*tfprotov6.SchemaAttribute, inherited bool) []string {
	var missing []string
	for _, attr := range attrs {
		attrPath := path + "." + attr.Name
		sensitive := inherited || attr.Sensitive
		if attr.NestedType != nil {
			missing = append(missing, unmarkedCredentials(attrPath, attr.NestedType.Attributes, sensitive)...)
			continue
		}
		if sensitive || !credentialAttributeName.MatchString(attr.Name) {
			continue
		}
		if _, ok := notSecret[attrPath]; ok {
			continue
		}
		missing = append(missing, attrPath)
	}
	return missing
}
