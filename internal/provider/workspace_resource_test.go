// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccWorkspaceResource(t *testing.T) {
	// Generate random names for each resource instance to avoid state conflicts
	randName1 := fmt.Sprintf("test-ws-%d", rand.Intn(10000))
	randName1Updated := fmt.Sprintf("%s-updated", randName1)

	randName2 := fmt.Sprintf("test-ws-prov-%d", rand.Intn(10000))
	randName2Updated := fmt.Sprintf("%s-updated", randName2)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccWorkspaceResourceConfig(accSpace.ID(), randName1, "development"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "name", randName1),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "space_id", accSpace.ID()),
				),
			},
			// Update and Read testing
			{
				Config: testAccWorkspaceResourceConfig(accSpace.ID(), randName1Updated, "production"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "name", randName1Updated),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "space_id", accSpace.ID()),
				),
			},
			// Create and Read testing
			{
				Config: testAccWorkspaceResourceWithSpaceInProviderConfig(accSpace.ID(), randName2, "qa"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "name", randName2),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "space_id", accSpace.ID()),
				),
			},
			// Update and Read testing
			{
				Config: testAccWorkspaceResourceWithSpaceInProviderConfig(accSpace.ID(), randName2Updated, "production"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "name", randName2Updated),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "space_id", accSpace.ID()),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccWorkspaceResourceConfig(spaceID, name, env string) string {
	return fmt.Sprintf(`
resource "mondoo_workspace" "test" {
  space_id         = %[1]q
  name             = %[2]q
  asset_selections = [
    {
      conditions = [
        {
          operator = "AND"
          key_value_condition = {
            field    = "LABELS"
            operator = "CONTAINS"
            values = [
              {
                key   = "environment"
                value = %[3]q
              }
            ]
          }
        }
      ]
    }
  ]
}
`, spaceID, name, env)
}

func TestAccWorkspaceResourceWithContacts(t *testing.T) {
	randName := fmt.Sprintf("test-ws-contacts-%d", rand.Intn(10000))

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create with contacts
			{
				Config: testAccWorkspaceResourceConfigWithContacts(accSpace.ID(), randName, []string{
					"alice@example.com",
					"bob@example.com",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "name", randName),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "contacts.#", "2"),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "contacts.0", "alice@example.com"),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "contacts.1", "bob@example.com"),
				),
			},
			// Update contacts
			{
				Config: testAccWorkspaceResourceConfigWithContacts(accSpace.ID(), randName, []string{
					"charlie@example.com",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "contacts.#", "1"),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "contacts.0", "charlie@example.com"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccWorkspaceResourceConfigWithContacts(spaceID, name string, contacts []string) string {
	contactsHCL := ""
	for _, c := range contacts {
		contactsHCL += fmt.Sprintf("    %q,\n", c)
	}
	return fmt.Sprintf(`
resource "mondoo_workspace" "test" {
  space_id = %[1]q
  name     = %[2]q

  contacts = [
%[4]s  ]

  asset_selections = [
    {
      conditions = [
        {
          operator = "AND"
          key_value_condition = {
            field    = "LABELS"
            operator = "CONTAINS"
            values = [
              {
                key   = "environment"
                value = "test"
              }
            ]
          }
        }
      ]
    }
  ]
}
`, spaceID, name, "", contactsHCL)
}

func testAccWorkspaceResourceWithSpaceInProviderConfig(spaceID, name, env string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}

resource "mondoo_workspace" "test" {
  name             = %[2]q
  asset_selections = [
    {
      conditions = [
        {
          operator = "AND"
          key_value_condition = {
            field    = "LABELS"
            operator = "CONTAINS"
            values = [
              {
                key   = "environment"
                value = %[3]q
              }
            ]
          }
        }
      ]
    }
  ]
}
`, spaceID, name, env)
}
