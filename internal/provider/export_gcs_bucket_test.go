// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccExportGCSBucketResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testExportGCSIntegration("bucket-export-integration", "my-bucket-name", accSpace.ID(), "CSV", "ServiceAccount_1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_export_gcs_bucket.test", "name", "bucket-export-integration"),
					resource.TestCheckResourceAttr("mondoo_export_gcs_bucket.test", "bucket_name", "my-bucket-name"),
					resource.TestCheckResourceAttr("mondoo_export_gcs_bucket.test", "space_id", accSpace.ID()),
				),
			},
			// Update and Read testing
			{
				Config: testExportGCSIntegration("bucket-export-integration-updated", "my-bucket-name-updated", accSpace.ID(), "JSONL", "ServiceAccount_2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_export_gcs_bucket.test", "name", "bucket-export-integration-updated"),
					resource.TestCheckResourceAttr("mondoo_export_gcs_bucket.test", "bucket_name", "my-bucket-name-updated"),
					resource.TestCheckResourceAttr("mondoo_export_gcs_bucket.test", "space_id", accSpace.ID()),
				),
			},
			// import testing
			{
				ResourceName: "mondoo_export_gcs_bucket.test",
				// setting the next two attributes allows the import to work in test, bc we use mrn instead of id
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return s.RootModule().Resources["mondoo_export_gcs_bucket.test"].Primary.Attributes["mrn"], nil
				},
				ImportStateVerifyIdentifierAttribute: "mrn",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"credentials"},
			},
		},
	})
}

func testExportGCSIntegration(name string, bucketName string, spaceId string, output string, serviceAccount string) string {
	return fmt.Sprintf(`
	resource "mondoo_export_gcs_bucket" "test" {
		name         = "%s"
		bucket_name  = "%s"
		space_id = "%s"
		export_format = "%s"
		credentials = {
			private_key = "%s"
		}
	}
	`, name, bucketName, spaceId, output, serviceAccount)
}
