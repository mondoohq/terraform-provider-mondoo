// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccIAMWorkloadIdentityBindingResource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccIAMWorkloadIdentityBindingResourceConfig(accSpace.ID(),
					"binding", "https://token.actions.githubusercontent.com", "repo:mondoohq/server:ref:refs/heads/main"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_iam_workload_identity_binding.test", "name", "binding"),
					resource.TestCheckResourceAttr("mondoo_iam_workload_identity_binding.test", "scope_mrn", accSpace.MRN()),
					resource.TestCheckResourceAttr("mondoo_iam_workload_identity_binding.test", "issuer_uri", "https://token.actions.githubusercontent.com"),
					resource.TestCheckResourceAttr("mondoo_iam_workload_identity_binding.test", "subject", "repo:mondoohq/server:ref:refs/heads/main"),
				),
			},
			// ImportState testing
			{
				ResourceName: "mondoo_iam_workload_identity_binding.test",
				// setting the next two attributes allows the import to work in test, bc we use mrn instead of id
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return s.RootModule().Resources["mondoo_iam_workload_identity_binding.test"].Primary.Attributes["mrn"], nil
				},
				ImportStateVerifyIdentifierAttribute: "mrn",
				ImportState:                          true,
				ImportStateVerify:                    true,
			},
			// Update is NOT allowed for this resource
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccIAMWorkloadIdentityBindingResourceConfig(spaceID, name, issuerURI, subject string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}

resource "mondoo_iam_workload_identity_binding" "test" {
  name       = %[2]q
  issuer_uri = %[3]q
  subject    = %[4]q
}
`, spaceID, name, issuerURI, subject)
}
