// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccAwsResourceWithCredential(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccAwsResourceCredentialConfig(accSpace.ID(), "aws-via-credential"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrPair(
						"mondoo_integration_aws.test", "credential_mrn",
						"mondoo_credential.test", "mrn",
					),
				),
			},
		},
	})
}

func testAccAwsResourceCredentialConfig(spaceID, intName string) string {
	return fmt.Sprintf(`
resource "mondoo_credential" "test" {
  space_id = %[1]q
  name     = "aws-access-key"

  aws = {
    access_key_id     = "AKIAIOSFODNN7EXAMPLE"
    secret_access_key = "wJalrXUtnFEMIK7MDENGbPxRfiCYEXAMPLEKEY"
    region            = "eu-central-1"
  }
}

resource "mondoo_integration_aws" "test" {
  space_id       = %[1]q
  name           = %[2]q
  credential_mrn = mondoo_credential.test.mrn
}
`, spaceID, intName)
}
