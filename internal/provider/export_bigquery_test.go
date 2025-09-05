// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccExportBigQueryResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testExportBigQueryIntegration("enterprise-demo-BigQuery", "project-id.dataset_id", accSpace.ID(), "hourly", true, "ServiceAccount_JSON_1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_export_bigquery.test", "name", "enterprise-demo-BigQuery"),
					resource.TestCheckResourceAttr("mondoo_export_bigquery.test", "dataset_id", "project-id.dataset_id"),
					resource.TestCheckResourceAttr("mondoo_export_bigquery.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_export_bigquery.test", "schedule", "hourly"),
					resource.TestCheckResourceAttr("mondoo_export_bigquery.test", "enabled", "true"),
				),
			},
			// Update and Read testing
			{
				Config: testExportBigQueryIntegration("enterprise-demo-BigQuery-updated", "project-id.dataset_id_new", accSpace.ID(), "daily", false, "ServiceAccount_JSON_2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_export_bigquery.test", "name", "enterprise-demo-BigQuery-updated"),
					resource.TestCheckResourceAttr("mondoo_export_bigquery.test", "dataset_id", "project-id.dataset_id_new"),
					resource.TestCheckResourceAttr("mondoo_export_bigquery.test", "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr("mondoo_export_bigquery.test", "schedule", "daily"),
					resource.TestCheckResourceAttr("mondoo_export_bigquery.test", "enabled", "false"),
				),
			},
			// import testing
			{
				ResourceName: "mondoo_export_bigquery.test",
				// setting the next two attributes allows the import to work in test, bc we use mrn instead of id
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return s.RootModule().Resources["mondoo_export_bigquery.test"].Primary.Attributes["mrn"], nil
				},
				ImportStateVerifyIdentifierAttribute: "mrn",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"service_account_key"},
			},
		},
	})
}

func testExportBigQueryIntegration(name string, datasetId string, spaceId string, schedule string, enabled bool, serviceAccountKey string) string {
	return fmt.Sprintf(`
	resource "mondoo_export_bigquery" "test" {
		name                = "%s"
		dataset_id          = "%s"
		space_id            = "%s"
		schedule            = "%s"
		enabled             = %t
		service_account_key = "%s"
	}
	`, name, datasetId, spaceId, schedule, enabled, serviceAccountKey)
}
