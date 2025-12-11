// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccGithubTicketingResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccGithubTicketingResourceConfig(accSpace.ID(), "test-github-integration", "mondoohq", "terraform-provider-mondoo"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "name", "test-github-integration"),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "owner", "mondoohq"),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "repository", "terraform-provider-mondoo"),
				),
			},
			// Test with auto_create and auto_close enabled
			{
				Config: testAccGithubTicketingResourceWithSpaceInProviderConfig(accSpace.ID(), "test-auto-enabled", "ghp_1234567890abcdef1234567890abcdef1234", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "name", "test-auto-enabled"),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "auto_create", "true"),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "auto_close", "true"),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "credentials.token", "ghp_1234567890abcdef1234567890abcdef1234"),
				),
			},
			// Update and Read testing
			{
				Config: testAccGithubTicketingResourceConfig(accSpace.ID(), "updated-integration", "mondoohq", "mondoo"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "name", "updated-integration"),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "owner", "mondoohq"),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "repository", "mondoo"),
				),
			},
			// Test with auto_create and auto_close disabled
			{
				Config: testAccGithubTicketingResourceWithSpaceInProviderConfig(accSpace.ID(), "test-auto-disabled", "ghp_1234567890abcdef1234567890abcdef1234", false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "name", "test-auto-disabled"),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "auto_create", "false"),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "auto_close", "false"),
					resource.TestCheckResourceAttr("mondoo_integration_github_ticketing.test", "credentials.token", "ghp_1234567890abcdef1234567890abcdef1234"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccGithubTicketingResourceConfig(spaceID, name, owner, repository string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_github_ticketing" "test" {
  space_id = %[1]q
  name      = %[2]q
  owner     = %[3]q
  repository = %[4]q

  auto_create = true
  auto_close  = true

  credentials = {
    token = "ghp_1234567890abcdef1234567890abcdef1234"
  }
}
`, spaceID, name, owner, repository)
}

func testAccGithubTicketingResourceWithSpaceInProviderConfig(spaceID, intName, token string, autoCreate, autoClose bool) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_github_ticketing" "test" {
  name       = %[2]q
  owner      = "mondoohq"
  repository = "terraform-provider-mondoo"

  auto_create = %[3]t
  auto_close  = %[4]t

  credentials = {
    token = %[5]q
  }
}
`, spaceID, intName, autoCreate, autoClose, token)
}
