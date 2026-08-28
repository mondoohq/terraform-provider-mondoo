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

// The schema is a subset of the SDK's kinds — gen/gen.go's credentialArms
// selects which ship — but it must never invent one. Every generated attribute
// has to correspond to a real arm of CredentialV2SecretInput.
func TestCredentialSecretAttributesAreRealArms(t *testing.T) {
	armType := reflect.TypeOf(mondoov1.CredentialV2SecretInput{})

	known := make(map[string]bool, armType.NumField())
	for i := 0; i < armType.NumField(); i++ {
		known[toSnakeCaseForTest(armType.Field(i).Name)] = true
	}

	for name := range credentialSecretAttributes() {
		if !known[name] {
			t.Errorf("attribute %q is not an arm of mondoov1.CredentialV2SecretInput", name)
		}
	}
}

// The kinds that have an integration binding must be among those shipped,
// otherwise credential_mrn on that integration refers to a credential the
// provider cannot create. Widening credentialArms beyond these is deliberate.
func TestCredentialSecretCoversTheBoundKinds(t *testing.T) {
	attrs := credentialSecretAttributes()

	for integration, kind := range map[string]string{
		"mondoo_integration_aws":    "aws",
		"mondoo_integration_slack":  "slack",
		"mondoo_integration_github": "github_pat",
	} {
		if _, ok := attrs[kind]; !ok {
			t.Errorf("%s binds to a %q credential, but that kind is not generated", integration, kind)
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

// Trap 3: fields the server derives from a secret — a GCP key's project_id and
// client_email, say — appear in fieldValues but are rejected as input, and must
// have no attribute to be written through.
//
// The guarantee is structural rather than a spot check: every attribute of
// every arm has to be a field of that arm's *input* type, which is where the
// generator reads them from. A derived name has no input field, so it can never
// acquire an attribute — for the kinds shipping today and for any kind added
// later.
func TestEveryAttributeIsAnInputField(t *testing.T) {
	attrs := credentialSecretAttributes()
	secretType := reflect.TypeOf(mondoov1.CredentialV2SecretInput{})

	for i := 0; i < secretType.NumField(); i++ {
		f := secretType.Field(i)

		attr, ok := attrs[toSnakeCaseForTest(f.Name)]
		if !ok {
			continue // not among the kinds this build ships
		}
		nested, ok := attr.(schema.SingleNestedAttribute)
		if !ok {
			t.Fatalf("arm %q is %T, want schema.SingleNestedAttribute", f.Name, attr)
		}

		armType := f.Type
		if armType.Kind() == reflect.Pointer {
			armType = armType.Elem()
		}
		inputFields := make(map[string]bool, armType.NumField())
		for j := 0; j < armType.NumField(); j++ {
			inputFields[toSnakeCaseForTest(armType.Field(j).Name)] = true
		}

		for fieldName := range nested.Attributes {
			if !inputFields[fieldName] {
				t.Errorf("%s.%s is not a field of %s; only input fields may be written through",
					toSnakeCaseForTest(f.Name), fieldName, armType.Name())
			}
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
