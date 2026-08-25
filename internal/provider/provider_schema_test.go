// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// The whole provider schema must validate the way it does at server start.
// Worth asserting because mondoo_credential's attributes are generated: a
// malformed one would otherwise surface only when a user runs terraform.
func TestProviderSchemaValidates(t *testing.T) {
	srv, err := providerserver.NewProtocol6WithError(New("test")())()
	if err != nil {
		t.Fatal(err)
	}

	resp, err := srv.GetProviderSchema(context.Background(), &tfprotov6.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError {
			t.Errorf("schema error: %s: %s", d.Summary, d.Detail)
		}
	}

	cred, ok := resp.ResourceSchemas["mondoo_credential"]
	if !ok {
		t.Fatal("mondoo_credential is not registered in provider.go")
	}

	// 15 hand-written attributes plus one per arm of CredentialV2SecretInput.
	const fixedAttributes = 15
	want := fixedAttributes + reflect.TypeOf(mondoov1.CredentialV2SecretInput{}).NumField()
	if got := len(cred.Block.Attributes); got != want {
		t.Errorf("mondoo_credential has %d attributes, want %d", got, want)
	}
}
