// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccResourceContactsResource(t *testing.T) {
	orgID, err := getOrgId()
	if err != nil {
		t.Fatal(err)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create contacts on a space
			{
				Config: testAccResourceContactsConfig(orgID, "contacts-test-space", []string{
					"alice@example.com",
					"bob@example.com",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_resource_contacts.test", "contacts.#", "2"),
					resource.TestCheckResourceAttr("mondoo_resource_contacts.test", "contacts.0", "alice@example.com"),
					resource.TestCheckResourceAttr("mondoo_resource_contacts.test", "contacts.1", "bob@example.com"),
				),
			},
			// Update contacts (replace)
			{
				Config: testAccResourceContactsConfig(orgID, "contacts-test-space", []string{
					"charlie@example.com",
				}),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_resource_contacts.test", "contacts.#", "1"),
					resource.TestCheckResourceAttr("mondoo_resource_contacts.test", "contacts.0", "charlie@example.com"),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccResourceContactsConfig(orgID, spaceName string, contacts []string) string {
	contactsHCL := ""
	for _, c := range contacts {
		contactsHCL += fmt.Sprintf("    %q,\n", c)
	}
	return fmt.Sprintf(`
resource "mondoo_space" "test" {
  org_id = %[1]q
  name   = %[2]q
}

resource "mondoo_resource_contacts" "test" {
  resource_mrn = mondoo_space.test.mrn
  contacts     = [
%[3]s  ]
}
`, orgID, spaceName, contactsHCL)
}
