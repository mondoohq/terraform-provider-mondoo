// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSpaceResource(t *testing.T) {
	orgID, err := getOrgId()
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccSpaceResourceConfig(orgID, "one"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_space.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_space.test", "org_id", orgID),
				),
			},
			// ImportState testing
			{
				ResourceName:      "mondoo_space.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing
			{
				Config: testAccSpaceResourceConfig(orgID, "two"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_space.test", "name", "two"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccSpaceResourceConfig(resourceOrgID string, name string) string {
	return fmt.Sprintf(`
resource "mondoo_space" "test" {
  org_id = %[1]q
  name = %[2]q
}
`, resourceOrgID, name)
}

func TestAccSpaceWithIDResource(t *testing.T) {
	orgID, err := getOrgId()
	if err != nil {
		t.Fatal(err)
	}

	minV := 1000
	maxV := 3000
	customSpaceID := "my-custom-space-id" + fmt.Sprint(rand.Intn(maxV-minV)+minV)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create Space with custom ID
			{
				Config: testAccSpaceResourceConfigWithID(orgID, customSpaceID, "one"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_space.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_space.test", "id", customSpaceID),
				),
			},
			// Update and Read testing
			{
				Config: testAccSpaceResourceConfigWithID(orgID, customSpaceID, "two"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_space.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_space.test", "id", customSpaceID),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccSpaceResourceConfigWithID(resourceOrgID string, id string, name string) string {
	return fmt.Sprintf(`
resource "mondoo_space" "test" {
  org_id = %[1]q
  id     = %[2]q
  name   = %[3]q
}
`, resourceOrgID, id, name)
}

func TestAccSpaceResourceWithSettings(t *testing.T) {
	orgID, err := getOrgId()
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing with all settings enabled
			{
				Config: testAccSpaceResourceConfigWithSettings(orgID, "one",
					true, true, true, true, true, true, true, true, true,
				),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_space.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_space.test", "org_id", orgID),
					resource.TestCheckResourceAttr("mondoo_space.test", "space_settings.terminated_assets_configuration.cleanup", "true"),
					resource.TestCheckResourceAttr("mondoo_space.test", "space_settings.garbage_collect_assets_configuration.enabled", "true"),
					resource.TestCheckResourceAttr("mondoo_space.test", "space_settings.exceptions_configuration.require_approval", "true"),
					resource.TestCheckResourceAttr("mondoo_space.test", "space_settings.exceptions_configuration.allow_indefinite_valid_until", "true"),
					resource.TestCheckResourceAttr("mondoo_space.test", "space_settings.exceptions_configuration.allow_self_approval", "true"),
				),
			},
			// ImportState testing
			{
				ResourceName:      "mondoo_space.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Update and Read testing with all settings disabled
			{
				Config: testAccSpaceResourceConfigWithSettings(orgID, "two",
					false, false, false, false, false, false, false, false, false,
				),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_space.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_space.test", "space_settings.terminated_assets_configuration.cleanup", "false"),
					resource.TestCheckResourceAttr("mondoo_space.test", "space_settings.garbage_collect_assets_configuration.enabled", "false"),
					resource.TestCheckResourceAttr("mondoo_space.test", "space_settings.exceptions_configuration.require_approval", "false"),
					resource.TestCheckResourceAttr("mondoo_space.test", "space_settings.exceptions_configuration.allow_indefinite_valid_until", "false"),
					resource.TestCheckResourceAttr("mondoo_space.test", "space_settings.exceptions_configuration.allow_self_approval", "false"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccSpaceResourceConfigWithSettings(resourceOrgID string, name string,
	terminated, unused, garbage, vuln, eol, cases,
	requireApproval, allowIndefinite, allowSelfApproval bool,
) string {
	return fmt.Sprintf(`
resource "mondoo_space" "test" {
  org_id = %[1]q
  name = %[2]q

  space_settings = {
    terminated_assets_configuration = {
      cleanup = %[3]t
    }
    unused_service_accounts_configuration = {
      cleanup = %[4]t
    }
    garbage_collect_assets_configuration = {
      enabled    = %[5]t
      after_days = 30
    }
    platform_vulnerability_configuration = {
      enabled = %[6]t
    }
    eol_assets_configuration = {
      enabled           = %[7]t
      months_in_advance = 6
    }
    cases_configuration = {
      auto_create        = %[8]t
      aggregation_window = 0
    }
    exceptions_configuration = {
      require_approval             = %[9]t
      allow_indefinite_valid_until = %[10]t
      allow_self_approval          = %[11]t
    }
  }
}
`, resourceOrgID, name,
		terminated, unused, garbage, vuln, eol, cases,
		requireApproval, allowIndefinite, allowSelfApproval,
	)
}
