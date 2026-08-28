// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccGithubResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing with space on resource
			{
				Config: testAccGithubResourceConfig(accSpace.ID(), "one", "lunalectric", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_github.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_github.test", "owner", "lunalectric"),
					resource.TestCheckResourceAttr("mondoo_integration_github.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_github.test", "discovery.terraform", "true"),
				),
			},
			// Update and Read testing with space in provider and explicit token
			{
				Config: testAccGithubResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "lunalectric", fakeGithubClassicToken()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_github.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_github.test", "owner", "lunalectric"),
					resource.TestCheckResourceAttr("mondoo_integration_github.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttrSet("mondoo_integration_github.test", "credentials.token"),
				),
			},
			// Plan-only step: setting force_replace should cause a non-empty plan (replacement)
			{
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
				Config:             testAccGithubResourceConfig(accSpace.ID(), "two", "lunalectric", true),
			},
			// Revert to previous config (without force_replace)
			{
				Config: testAccGithubResourceConfig(accSpace.ID(), "two", "lunalectric", false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_github.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_github.test", "owner", "lunalectric"),
					resource.TestCheckResourceAttr("mondoo_integration_github.test", "space_id", accSpace.ID()),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

// fakeGithubClassicToken constructs a valid-looking classic token without embedding the literal pattern in source.
func fakeGithubClassicToken() string {
	return "gh" + "p_" + strings.Repeat("A", 36)
}

func testAccGithubResourceConfig(spaceID, intName, owner string, forceReplace bool) string {
	forceReplaceLine := ""
	if forceReplace {
		forceReplaceLine = "\n\tforce_replace = true"
	}
	tok := fakeGithubClassicToken()
	return fmt.Sprintf(`
resource "mondoo_integration_github" "test" {
  space_id = %q
  name  = %q
  owner = %q

  discovery = {
    terraform     = true
    k8s_manifests = true
  }%s

  credentials = {
    token = %q
  }
}
`, spaceID, intName, owner, forceReplaceLine, tok)
}

func testAccGithubResourceWithSpaceInProviderConfig(spaceID, intName, owner, token string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %q
}
resource "mondoo_integration_github" "test" {
  name  = %q
  owner = %q

  discovery = {
    terraform     = true
    k8s_manifests = true
  }

  credentials = {
    token = %q
  }
}
`, spaceID, intName, owner, token)
}

func TestAccGithubResourceWithCredential(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccGithubResourceCredentialConfig(accSpace.ID(), "github-via-credential"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"mondoo_integration_github.test", "credential_mrn",
						"mondoo_credential.test", "mrn",
					),
				),
			},
		},
	})
}

func testAccGithubResourceCredentialConfig(spaceID, intName string) string {
	return fmt.Sprintf(`
resource "mondoo_credential" "test" {
  space_id = %[1]q
  name     = "github-scanner"

  github_pat = {
    token = "ghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
  }
}

resource "mondoo_integration_github" "test" {
  space_id       = %[1]q
  name           = %[2]q
  owner          = "mondoohq"
  credential_mrn = mondoo_credential.test.mrn
}
`, spaceID, intName)
}
