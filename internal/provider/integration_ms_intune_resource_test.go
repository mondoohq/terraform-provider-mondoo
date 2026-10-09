// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestIntegrationMsIntuneGetConfigurationOptions_Options checks that the
// options are always sent, so turning one off reaches the API as an explicit
// false instead of being omitted (which the API treats as "unchanged").
func TestIntegrationMsIntuneGetConfigurationOptions_Options(t *testing.T) {
	for _, tc := range []struct {
		name          string
		importDevices types.Bool
		aiDiscovery   types.Bool
		wantImport    bool
		wantAi        bool
	}{
		{name: "unset", importDevices: types.BoolNull(), aiDiscovery: types.BoolNull()},
		{name: "false", importDevices: types.BoolValue(false), aiDiscovery: types.BoolValue(false)},
		{name: "import only", importDevices: types.BoolValue(true), aiDiscovery: types.BoolValue(false), wantImport: true},
		{name: "both", importDevices: types.BoolValue(true), aiDiscovery: types.BoolValue(true), wantImport: true, wantAi: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := integrationMsIntuneResourceModel{
				TenantId:      types.StringValue("a-tenant-id"),
				ClientId:      types.StringValue("a-client-id"),
				ImportDevices: tc.importDevices,
				AiDiscovery:   tc.aiDiscovery,
			}

			opts := m.GetConfigurationOptions()
			require.NotNil(t, opts)
			assert.EqualValues(t, "a-tenant-id", opts.TenantId)
			assert.EqualValues(t, "a-client-id", opts.ClientId)
			require.NotNil(t, opts.ImportDevices)
			assert.EqualValues(t, tc.wantImport, *opts.ImportDevices)
			require.NotNil(t, opts.AiDiscovery)
			assert.EqualValues(t, tc.wantAi, *opts.AiDiscovery)
			// no client secret configured => none sent
			assert.Nil(t, opts.Password)
		})
	}
}

func TestAccMsIntuneIntegrationResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccMsIntuneIntegrationResourceConfig(accSpace.ID(), "one", "a-tenant-id", "a-client-id", "a-client-secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "name", "one"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "tenant_id", "a-tenant-id"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "client_id", "a-client-id"),
					// options default to off
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "import_devices", "false"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "ai_discovery", "false"),
				),
			},
			{
				Config: testAccMsIntuneIntegrationResourceWithSpaceInProviderConfig(accSpace.ID(), "two", "b-tenant-id", "b-client-id", "b-client-secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "name", "two"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "tenant_id", "b-tenant-id"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "client_id", "b-client-id"),
				),
			},
			// Update and Read testing
			{
				Config: testAccMsIntuneIntegrationResourceConfig(accSpace.ID(), "three", "new-tenant-id", "new-client-id", "new-client-secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "name", "three"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "tenant_id", "new-tenant-id"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "client_id", "new-client-id"),
				),
			},
			// Enable the options
			{
				Config: testAccMsIntuneIntegrationResourceWithOptionsConfig(accSpace.ID(), "three", "new-tenant-id", "new-client-id", "new-client-secret", true, true),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "import_devices", "true"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "ai_discovery", "true"),
				),
			},
			// Disable them again: an explicit false must reach the API
			{
				Config: testAccMsIntuneIntegrationResourceWithOptionsConfig(accSpace.ID(), "three", "new-tenant-id", "new-client-id", "new-client-secret", false, false),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "import_devices", "false"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "ai_discovery", "false"),
				),
			},
			// Removing them from the config falls back to the default (false)
			{
				Config: testAccMsIntuneIntegrationResourceConfig(accSpace.ID(), "three", "new-tenant-id", "new-client-id", "new-client-secret"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "import_devices", "false"),
					resource.TestCheckResourceAttr("mondoo_integration_ms_intune.test", "ai_discovery", "false"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccMsIntuneIntegrationResourceConfig(spaceID, intName, tenantID, clientID, clientSecret string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_ms_intune" "test" {
  space_id  = %[1]q
  name      = %[2]q
  tenant_id = %[3]q
  client_id = %[4]q
  credentials = {
    client_secret = %[5]q
  }
}
`, spaceID, intName, tenantID, clientID, clientSecret)
}

func testAccMsIntuneIntegrationResourceWithSpaceInProviderConfig(spaceID, intName, tenantID, clientID, clientSecret string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}
resource "mondoo_integration_ms_intune" "test" {
  name      = %[2]q
  tenant_id = %[3]q
  client_id = %[4]q
  credentials = {
    client_secret = %[5]q
  }
}
`, spaceID, intName, tenantID, clientID, clientSecret)
}

func testAccMsIntuneIntegrationResourceWithOptionsConfig(spaceID, intName, tenantID, clientID, clientSecret string, importDevices, aiDiscovery bool) string {
	return fmt.Sprintf(`
resource "mondoo_integration_ms_intune" "test" {
  space_id       = %[1]q
  name           = %[2]q
  tenant_id      = %[3]q
  client_id      = %[4]q
  import_devices = %[6]t
  ai_discovery   = %[7]t
  credentials = {
    client_secret = %[5]q
  }
}
`, spaceID, intName, tenantID, clientID, clientSecret, importDevices, aiDiscovery)
}
