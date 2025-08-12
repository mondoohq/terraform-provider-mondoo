// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccS3ExportResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testS3ExportIntegration("s3-export-integration", "my-mondoo-exports", "us-west-2", accSpace.ID(), "jsonl", "AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_export_s3.test", "name", "s3-export-integration"),
					resource.TestCheckResourceAttr("mondoo_export_s3.test", "bucket_name", "my-mondoo-exports"),
					resource.TestCheckResourceAttr("mondoo_export_s3.test", "region", "us-west-2"),
					resource.TestCheckResourceAttr("mondoo_export_s3.test", "export_format", "jsonl"),
					resource.TestCheckResourceAttr("mondoo_export_s3.test", "space_id", accSpace.ID()),
				),
			},
			// Update and Read testing
			{
				Config: testS3ExportIntegration("s3-export-integration-updated", "my-mondoo-exports-updated", "us-east-1", accSpace.ID(), "CSV", "AKIAIOSFODNN7EXAMPLE2", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_export_s3.test", "name", "s3-export-integration-updated"),
					resource.TestCheckResourceAttr("mondoo_export_s3.test", "bucket_name", "my-mondoo-exports-updated"),
					resource.TestCheckResourceAttr("mondoo_export_s3.test", "region", "us-east-1"),
					resource.TestCheckResourceAttr("mondoo_export_s3.test", "export_format", "CSV"),
					resource.TestCheckResourceAttr("mondoo_export_s3.test", "space_id", accSpace.ID()),
				),
			},
			// Import testing
			{
				ResourceName: "mondoo_export_s3.test",
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return s.RootModule().Resources["mondoo_export_s3.test"].Primary.Attributes["mrn"], nil
				},
				ImportStateVerifyIdentifierAttribute: "mrn",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIgnore:              []string{"credentials"},
			},
		},
	})
}

func testS3ExportIntegration(name string, bucket string, region string, spaceId string, output string, accessKey string, secretKey string) string {
	return fmt.Sprintf(`
	resource "mondoo_export_s3" "test" {
		name        = "%s"
		bucket_name = "%s"
		region      = "%s"
		space_id    = "%s"
		export_format = "%s"
		credentials = {
			key = {
				access_key = "%s"
				secret_key = "%s"
			}
		}
	}
	`, name, bucket, region, spaceId, output, accessKey, secretKey)
}
