// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/shurcooL/graphql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMs365ConfigurationOptions(t *testing.T) {
	base := func() integrationMs365ResourceModel {
		return integrationMs365ResourceModel{
			TenantId: types.StringValue("tenant-1"),
			ClientId: types.StringValue("client-1"),
		}
	}

	t.Run("wif mode sends useWif and no certificate", func(t *testing.T) {
		m := base()
		m.UseWif = types.BoolValue(true)

		var diags diag.Diagnostics
		opts, ok := ms365ConfigurationOptions(m, &diags)

		require.True(t, ok)
		assert.False(t, diags.HasError())
		require.NotNil(t, opts.UseWif)
		assert.EqualValues(t, true, bool(*opts.UseWif))
		// The API rejects a WIF request that also carries a certificate.
		assert.Nil(t, opts.Certificate)
		assert.EqualValues(t, "tenant-1", opts.TenantId)
		assert.EqualValues(t, "client-1", opts.ClientId)
	})

	t.Run("wif mode wins over a stale credentials block", func(t *testing.T) {
		m := base()
		m.UseWif = types.BoolValue(true)
		m.Credential = &integrationMs365CredentialModel{PEMFile: types.StringValue("pem")}

		var diags diag.Diagnostics
		opts, ok := ms365ConfigurationOptions(m, &diags)

		require.True(t, ok)
		assert.Nil(t, opts.Certificate)
	})

	t.Run("certificate mode sends the pem and no useWif", func(t *testing.T) {
		m := base()
		m.Credential = &integrationMs365CredentialModel{PEMFile: types.StringValue("pem-body")}

		var diags diag.Diagnostics
		opts, ok := ms365ConfigurationOptions(m, &diags)

		require.True(t, ok)
		assert.False(t, diags.HasError())
		assert.Nil(t, opts.UseWif)
		require.NotNil(t, opts.Certificate)
		assert.EqualValues(t, "pem-body", string(*opts.Certificate))
	})

	t.Run("explicit use_wif=false without credentials errors instead of panicking", func(t *testing.T) {
		// ExactlyOneOf treats an explicit false as configured, so this reaches
		// the builder with a nil credentials model.
		m := base()
		m.UseWif = types.BoolValue(false)

		var diags diag.Diagnostics
		opts, ok := ms365ConfigurationOptions(m, &diags)

		assert.False(t, ok)
		assert.Nil(t, opts)
		assert.True(t, diags.HasError())
	})
}

// TestMs365GraphQLIssuerFieldCasing pins the GraphQL spelling of the MS365 WIF
// issuer field. The MS365 schema spells it `wifIssuerURL` while Azure spells its
// own `wifIssuerUrl`, and the library's untagged name derivation lowercases the
// trailing initialism — so dropping the struct tag silently queries a field the
// server does not have.
func TestMs365GraphQLIssuerFieldCasing(t *testing.T) {
	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Query string `json:"query"`
		}
		_ = json.Unmarshal(body, &req)
		captured = req.Query
		_, _ = fmt.Fprint(w, `{"data":{"clientIntegration":{"integration":{}}}}`)
	}))
	defer srv.Close()

	// Same query shape as ExtendedGqlClient.GetClientIntegration.
	var q struct {
		ClientIntegration ClientIntegration `graphql:"clientIntegration(input: {mrn: $mrn})"`
	}
	err := graphql.NewClient(srv.URL, srv.Client()).
		Query(context.Background(), &q, map[string]interface{}{"mrn": graphql.String("mrn")})
	require.NoError(t, err)

	ms365 := regexp.MustCompile(`\.\.\. on Ms365ConfigurationOptions\{[^}]*\}`).FindString(captured)
	require.NotEmpty(t, ms365, "MS365 fragment missing from query: %s", captured)
	assert.Contains(t, ms365, "wifIssuerURL")
	assert.Contains(t, ms365, "wifSubject")
	assert.Contains(t, ms365, "useWif")

	azure := regexp.MustCompile(`\.\.\. on AzureConfigurationOptions\{[^}]*\}`).FindString(captured)
	require.NotEmpty(t, azure, "Azure fragment missing from query: %s", captured)
	assert.Contains(t, azure, "wifIssuerUrl")
}

func TestAccMs365IntegrationResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Certificate mode: create and read
			{
				Config: testAccMs365IntegrationResourceCertConfig(accSpace.ID(), "cert-one", "abcd1234567890"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_ms365.test", "name", "cert-one"),
					resource.TestCheckResourceAttr("mondoo_integration_ms365.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_ms365.test", "credentials.pem_file", "abcd1234567890"),
					resource.TestCheckNoResourceAttr("mondoo_integration_ms365.test", "use_wif"),
				),
			},
			// Certificate mode: update
			{
				Config: testAccMs365IntegrationResourceCertConfig(accSpace.ID(), "cert-two", "abcd1234567890"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_ms365.test", "name", "cert-two"),
				),
			},
		},
	})
}

func TestAccMs365IntegrationResourceWif(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// WIF mode: create and read. wif_subject is computed by the server
			// from the integration MRN, so it must come back populated.
			{
				Config: testAccMs365IntegrationResourceWifConfig(accSpace.ID(), "wif-one"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_ms365.wif", "name", "wif-one"),
					resource.TestCheckResourceAttr("mondoo_integration_ms365.wif", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_integration_ms365.wif", "use_wif", "true"),
					resource.TestCheckNoResourceAttr("mondoo_integration_ms365.wif", "credentials"),
					resource.TestCheckResourceAttrSet("mondoo_integration_ms365.wif", "wif_subject"),
				),
			},
			// WIF mode: rename keeps the computed subject stable.
			{
				Config: testAccMs365IntegrationResourceWifConfig(accSpace.ID(), "wif-two"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_integration_ms365.wif", "name", "wif-two"),
					resource.TestCheckResourceAttr("mondoo_integration_ms365.wif", "use_wif", "true"),
					resource.TestCheckResourceAttrSet("mondoo_integration_ms365.wif", "wif_subject"),
				),
			},
			// Import: the auth mode is recovered from the API response.
			{
				ResourceName: "mondoo_integration_ms365.wif",
				// this resource keys off mrn rather than id, so point the
				// import at it explicitly
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return s.RootModule().Resources["mondoo_integration_ms365.wif"].Primary.Attributes["mrn"], nil
				},
				ImportStateVerifyIdentifierAttribute: "mrn",
				ImportState:                          true,
				ImportStateVerify:                    true,
			},
		},
	})
}

func testAccMs365IntegrationResourceCertConfig(spaceID, intName, pemFile string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_ms365" "test" {
  space_id  = %[1]q
  name      = %[2]q
  tenant_id = "ffffffff-ffff-ffff-ffff-ffffffffffff"
  client_id = "ffffffff-ffff-ffff-ffff-ffffffffffff"
  credentials = {
    pem_file = %[3]q
  }
}
`, spaceID, intName, pemFile)
}

func testAccMs365IntegrationResourceWifConfig(spaceID, intName string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_ms365" "wif" {
  space_id  = %[1]q
  name      = %[2]q
  tenant_id = "ffffffff-ffff-ffff-ffff-ffffffffffff"
  client_id = "ffffffff-ffff-ffff-ffff-ffffffffffff"
  use_wif   = true
}
`, spaceID, intName)
}
