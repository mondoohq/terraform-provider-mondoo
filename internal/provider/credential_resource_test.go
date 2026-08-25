// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCredentialResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read
			{
				Config: testAccCredentialResourceConfig(accSpace.ID(), "cred-one", "ghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_credential.test", "name", "cred-one"),
					resource.TestCheckResourceAttr("mondoo_credential.test", "kind", "GITHUB_PAT"),
					resource.TestCheckResourceAttrSet("mondoo_credential.test", "mrn"),
					resource.TestCheckResourceAttrSet("mondoo_credential.test", "owner_mrn"),
					resource.TestCheckResourceAttrSet("mondoo_credential.test", "health_status"),
				),
			},
			// Rename in place
			{
				Config: testAccCredentialResourceConfig(accSpace.ID(), "cred-two", "ghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_credential.test", "name", "cred-two"),
				),
			},
			// Rotate the secret
			{
				Config: testAccCredentialResourceConfig(accSpace.ID(), "cred-two", "ghp_bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_credential.test", "kind", "GITHUB_PAT"),
				),
			},
			// Import by MRN. The secret cannot be imported — no read returns it
			// — and import records scope_mrn where the config used space_id.
			{
				ResourceName:            "mondoo_credential.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"github_pat", "space_id", "scope_mrn"},
			},
			// Delete happens automatically at the end of the case
		},
	})
}

// Changing which kind attribute is set must be refused at plan time with the
// migration recipe, never planned as a replacement.
func TestAccCredentialResourceKindChangeRefused(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCredentialResourceConfig(accSpace.ID(), "cred-kind", "ghp_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
			},
			{
				Config:      testAccCredentialSlackConfig(accSpace.ID(), "cred-kind"),
				ExpectError: regexp.MustCompile(`Credential kind cannot be changed`),
			},
		},
	})
}

// An organization-owned credential, addressed by scope_mrn rather than
// space_id. Acceptance tests already require an organization service account.
func TestAccCredentialResourceOrgScope(t *testing.T) {
	// getOrgId() reads acceptance-test credentials, and it runs before
	// resource.Test() gets its chance to skip on TF_ACC — so guard it here or
	// this test fails rather than skips during an ordinary `go test ./...`.
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Acceptance tests skipped unless env 'TF_ACC' set")
	}

	orgID, err := getOrgId()
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCredentialOrgScopeConfig(orgPrefix+orgID, "cred-org"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_credential.test", "scope_mrn", orgPrefix+orgID),
					resource.TestCheckResourceAttr("mondoo_credential.test", "owner_mrn", orgPrefix+orgID),
					resource.TestCheckResourceAttr("mondoo_credential.test", "kind", "SLACK"),
				),
			},
		},
	})
}

// A credential an integration references reports that integration and the slot
// it fills, and cannot be deleted while the reference stands.
func TestAccCredentialResourceUsagesWhileInUse(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccCredentialWithSlackIntegrationConfig(accSpace.ID()),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("mondoo_integration_slack.test", "credential_mrn"),
					resource.TestCheckResourceAttr("mondoo_credential.test", "usages.#", "1"),
					resource.TestCheckResourceAttrPair(
						"mondoo_credential.test", "usages.0.mrn",
						"mondoo_integration_slack.test", "mrn",
					),
				),
			},
		},
	})
}

func testAccCredentialResourceConfig(spaceID, name, token string) string {
	return fmt.Sprintf(`
resource "mondoo_credential" "test" {
  space_id = %[1]q
  name     = %[2]q

  github_pat = {
    token = %[3]q
  }
}
`, spaceID, name, token)
}

func testAccCredentialSlackConfig(spaceID, name string) string {
	return fmt.Sprintf(`
resource "mondoo_credential" "test" {
  space_id = %[1]q
  name     = %[2]q

  slack = {
    bot_token = "xoxb-1234567890abc"
  }
}
`, spaceID, name)
}

func testAccCredentialOrgScopeConfig(scopeMrn, name string) string {
	return fmt.Sprintf(`
resource "mondoo_credential" "test" {
  scope_mrn = %[1]q
  name      = %[2]q

  slack = {
    bot_token = "xoxb-1234567890abc"
  }
}
`, scopeMrn, name)
}

func testAccCredentialWithSlackIntegrationConfig(spaceID string) string {
	return fmt.Sprintf(`
resource "mondoo_credential" "test" {
  space_id = %[1]q
  name     = "slack-bot"

  slack = {
    bot_token = "xoxb-1234567890abc"
  }
}

resource "mondoo_integration_slack" "test" {
  space_id       = %[1]q
  name           = "slack-via-credential"
  credential_mrn = mondoo_credential.test.mrn
}
`, spaceID)
}
