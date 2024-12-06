// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccJiraResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccJiraResourceConfig(accSpace.ID(), "one", "https://your-instance.atlassian.net", "jira.owner@email.com", "MONDOO"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_jira.test", "host", "https://your-instance.atlassian.net"),
					resource.TestCheckResourceAttr("mondoo_integration_jira.test", "email", "jira.owner@email.com"),
					resource.TestCheckResourceAttr("mondoo_integration_jira.test", "default_project", "MONDOO"),
				),
			},
			{
				Config: testAccJiraResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "abctoken12345", true, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_jira.test", "credentials.0.token", "abctoken12345"),
					resource.TestCheckResourceAttr("mondoo_integration_jira.test", "auto_create", "true"),
					resource.TestCheckResourceAttr("mondoo_integration_jira.test", "auto_close", "false"),
				),
			},
			// ImportState testing
			// @afiune this doesn't work since most of our resources doesn't have the `id` attribute
			// if we add it, instead of the `mrn` or as a copy, this import test will work
			// {
			// ResourceName:      "mondoo_integration_shodan.test",
			// ImportState:       true,
			// ImportStateVerify: true,
			// },
			// Update and Read testing
			{
				Config: testAccJiraResourceConfig(accSpace.ID(), "one", "https://your-instance.atlassian.net", "jira.owner@email.com", "MONDOO"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_jira.test", "host", "https://your-instance.atlassian.net"),
					resource.TestCheckResourceAttr("mondoo_integration_jira.test", "email", "jira.owner@email.com"),
					resource.TestCheckResourceAttr("mondoo_integration_jira.test", "default_project", "MONDOO"),
				),
			},
			{
				Config: testAccJiraResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "abctoken12345", true, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_shodan.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_jira.test", "credentials.0.token", "abctoken12345"),
					resource.TestCheckResourceAttr("mondoo_integration_jira.test", "auto_create", "true"),
					resource.TestCheckResourceAttr("mondoo_integration_jira.test", "auto_close", "false"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccJiraResourceConfig(spaceID, intName, host, email, defaultProject string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_shodan" "test" {
	space_id = %[1]q
  name = %[2]q
  targets = %[3]q
	credentials = {
	  token = "abcd1234567890"
	}
}
resource "mondoo_integration_jira" "test" {
  space_id = %[1]q
  name  = %[2]q
  host  = %[3]q
  email = %[4]q
  default_project = %[5]q

  auto_create = true
  auto_close  = true

  credentials = {
    token = "abcd1234567890"
  }
}
`, spaceID, intName, host, email, defaultProject)
}

func testAccJiraResourceWithSpaceInProviderConfig(spaceID, intName, token string, autoCreate, autoClose bool) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_jira" "test" {
  name  = %[2]q
  host  = "https://your-instance.atlassian.net"
  email = "jira.owner@email.com"
  default_project = "MONDOO"

  auto_create = %[4]t
  auto_close  = %[5]t

  credentials = {
    token = %[3]q
  }
}
`, spaceID, intName, token, autoCreate, autoClose)
}
