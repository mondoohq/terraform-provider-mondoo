// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Every arm of CredentialV2SecretInput must have an attribute. Derived from the
// SDK type rather than a hand-written list, so a new kind fails this test until
// the generator is re-run.
func TestCredentialSecretAttributesCoverEveryArm(t *testing.T) {
	attrs := credentialSecretAttributes()
	armType := reflect.TypeOf(mondoov1.CredentialV2SecretInput{})

	if len(attrs) != armType.NumField() {
		t.Fatalf("got %d attributes, want %d arms", len(attrs), armType.NumField())
	}
	for i := 0; i < armType.NumField(); i++ {
		name := toSnakeCaseForTest(armType.Field(i).Name)
		if _, ok := attrs[name]; !ok {
			t.Errorf("no attribute for arm %q (%s)", name, armType.Field(i).Name)
		}
	}
}

// The whole sensitivity rule: no exceptions, nothing opted out.
func TestCredentialSecretFieldsAreAllSensitive(t *testing.T) {
	for armName, attr := range credentialSecretAttributes() {
		nested, ok := attr.(schema.SingleNestedAttribute)
		if !ok {
			t.Fatalf("arm %q is %T, want schema.SingleNestedAttribute", armName, attr)
		}
		if len(nested.Attributes) == 0 {
			t.Errorf("arm %q has no fields", armName)
		}
		for fieldName, field := range nested.Attributes {
			if !field.IsSensitive() {
				t.Errorf("%s.%s is not Sensitive; every credential field must be", armName, fieldName)
			}
		}
	}
}

func TestCredentialSecretPathsCoverEveryAttribute(t *testing.T) {
	attrs := credentialSecretAttributes()
	paths := credentialSecretPaths()
	if len(paths) != len(attrs) {
		t.Fatalf("got %d paths, want %d attributes", len(paths), len(attrs))
	}
	seen := map[string]bool{}
	for _, p := range paths {
		seen[p.String()] = true
	}
	for name := range attrs {
		if !seen[name] {
			t.Errorf("no ExactlyOneOf path for attribute %q", name)
		}
	}
}

// Trap 2: rotation replaces the whole payload, so every field of the set arm is
// sent every time, including the optional ones.
func TestBuildSecretInputSendsEveryFieldOfTheArm(t *testing.T) {
	m := &credentialResourceModel{
		GithubPat: &credentialGithubPatModel{
			Token:   types.StringValue("ghp_secret"),
			BaseUrl: types.StringValue("https://github.acme.com"),
		},
	}

	in, err := m.BuildSecretInput()
	if err != nil {
		t.Fatalf("BuildSecretInput: %v", err)
	}
	if in.GithubPat == nil {
		t.Fatal("GithubPat arm not set")
	}
	if got := string(in.GithubPat.Token); got != "ghp_secret" {
		t.Errorf("Token = %q, want %q", got, "ghp_secret")
	}
	if in.GithubPat.BaseUrl == nil {
		t.Fatal("BaseUrl was dropped; rotation would reset the enterprise URL")
	}
	if got := string(*in.GithubPat.BaseUrl); got != "https://github.acme.com" {
		t.Errorf("BaseUrl = %q, want %q", got, "https://github.acme.com")
	}
	if in.Aws != nil || in.Slack != nil {
		t.Error("more than one arm set")
	}
}

func TestBuildSecretInputOmittedOptionalFieldIsNil(t *testing.T) {
	m := &credentialResourceModel{
		GithubPat: &credentialGithubPatModel{
			Token:   types.StringValue("ghp_secret"),
			BaseUrl: types.StringNull(),
		},
	}

	in, err := m.BuildSecretInput()
	if err != nil {
		t.Fatalf("BuildSecretInput: %v", err)
	}
	if in.GithubPat.BaseUrl != nil {
		t.Errorf("BaseUrl = %v, want nil for an unset optional field", *in.GithubPat.BaseUrl)
	}
}

func TestBuildSecretInputAws(t *testing.T) {
	m := &credentialResourceModel{
		Aws: &credentialAwsModel{
			AccessKeyId:     types.StringValue("AKIAIOSFODNN7EXAMPLE"),
			SecretAccessKey: types.StringValue("wJalrXUtnFEMI"),
			Region:          types.StringValue("eu-central-1"),
		},
	}

	in, err := m.BuildSecretInput()
	if err != nil {
		t.Fatalf("BuildSecretInput: %v", err)
	}
	if in.Aws == nil {
		t.Fatal("Aws arm not set")
	}
	if got := string(in.Aws.AccessKeyId); got != "AKIAIOSFODNN7EXAMPLE" {
		t.Errorf("AccessKeyId = %q", got)
	}
	if in.Aws.Region == nil || string(*in.Aws.Region) != "eu-central-1" {
		t.Error("Region was dropped; rotation would reset it to us-east-1")
	}
}

func TestBuildSecretInputRejectsZeroOrTwoArms(t *testing.T) {
	if _, err := (&credentialResourceModel{}).BuildSecretInput(); err == nil {
		t.Error("want an error when no arm is set")
	}

	two := &credentialResourceModel{
		GithubPat: &credentialGithubPatModel{Token: types.StringValue("a")},
		Slack:     &credentialSlackModel{BotToken: types.StringValue("b")},
	}
	if _, err := two.BuildSecretInput(); err == nil {
		t.Error("want an error when two arms are set")
	}
}

func TestSecretKind(t *testing.T) {
	m := &credentialResourceModel{Slack: &credentialSlackModel{BotToken: types.StringValue("xoxb-1")}}

	kind, err := m.SecretKind()
	if err != nil {
		t.Fatalf("SecretKind: %v", err)
	}
	if kind != mondoov1.CredentialV2KindSlack {
		t.Errorf("SecretKind() = %q, want %q", kind, mondoov1.CredentialV2KindSlack)
	}
}

// Trap 3: derivedFields must have no attribute to be written through. The
// server parses project_id and client_email out of a GCP key and rejects them
// as input; the generator emits input arms only, so they cannot appear.
func TestNoAttributeForDerivedFields(t *testing.T) {
	gcp, ok := credentialSecretAttributes()["gcp_service_account"].(schema.SingleNestedAttribute)
	if !ok {
		t.Fatal("gcp_service_account is not a SingleNestedAttribute")
	}
	for _, derived := range []string{"project_id", "client_email"} {
		if _, exists := gcp.Attributes[derived]; exists {
			t.Errorf("gcp_service_account has an attribute for derived field %q", derived)
		}
	}
}

// toSnakeCaseForTest mirrors gen/gen.go's toSnakeCase. It is duplicated rather
// than imported because gen is package main.
func toSnakeCaseForTest(str string) string {
	snake := testMatchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = testMatchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

var (
	testMatchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
	testMatchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)
