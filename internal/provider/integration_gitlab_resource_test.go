// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccGitLabResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccGitLabResourceConfig(accSpace.ID(), "one", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_gitlab.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_gitlab.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_gitlab.test", "discovery.0.groups", "true"),
				),
			},
			{
				Config: testAccGitLabResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "abctoken12345"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_gitlab.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_gitlab.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_gitlab.test", "credentials.0.token", "abctoken12345"),
				),
			},
			// ImportState testing
			// @afiune this doesn't work since most of our resources doesn't have the `id` attribute
			// if we add it, instead of the `mrn` or as a copy, this import test will work
			// {
			// ResourceName:      "mondoo_integration_gitlab.test",
			// ImportState:       true,
			// ImportStateVerify: true,
			// },
			// Update and Read testing
			{
				Config: testAccGitLabResourceConfig(accSpace.ID(), "one", true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_gitlab.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_gitlab.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_gitlab.test", "discovery.0.groups", "true"),
				),
			},
			{
				Config: testAccGitLabResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "abctoken12345"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_gitlab.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_gitlab.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_gitlab.test", "credentials.0.token", "abctoken12345"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccGitLabResourceConfig(spaceID, intName string, discoveryGroup bool) string {
	return fmt.Sprintf(`
resource "mondoo_integration_gitlab" "test" {
	space_id = %[1]q
  	name = %[2]q
	base_url = "https://my-self-hosted-gitlab.com"
  	group    = "my-group"
	discovery = {
	  groups        = %[3]t
	  projects      = true
	  terraform     = true
	  k8s_manifests = true
	}
	credentials = {
	  token = "abcd1234567890"
	}
}
`, spaceID, intName, discoveryGroup)
}

func testAccGitLabResourceWithSpaceInProviderConfig(spaceID, intName, token string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_gitlab" "test" {
  	name = %[2]q
	base_url = "https://my-self-hosted-gitlab.com"
  	group    = "my-group"
	discovery = {
	  groups        = true
	  projects      = true
	  terraform     = true
	  k8s_manifests = true
	}
	credentials = {
	  token = %[3]q
	}
}
`, spaceID, intName, token)
}
