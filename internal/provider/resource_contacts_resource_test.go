// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mondoov1 "go.mondoo.com/mondoo-go"
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
			// Add links alongside contacts
			{
				Config: testAccResourceContactsWithLinksConfig(orgID, "contacts-test-space"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_resource_contacts.test", "contacts.#", "1"),
					resource.TestCheckResourceAttr("mondoo_resource_contacts.test", "contacts.0", "charlie@example.com"),
					resource.TestCheckResourceAttr("mondoo_resource_contacts.test", "links.#", "2"),
					resource.TestCheckResourceAttr("mondoo_resource_contacts.test", "links.0.name", "Runbook"),
					resource.TestCheckResourceAttr("mondoo_resource_contacts.test", "links.0.url", "https://wiki.example.com/runbook"),
					resource.TestCheckResourceAttr("mondoo_resource_contacts.test", "links.1.name", "Dashboard"),
					resource.TestCheckResourceAttr("mondoo_resource_contacts.test", "links.1.url", "https://grafana.example.com/d/prod"),
				),
			},
			// Import picks up both contacts and links
			{
				ResourceName:                         "mondoo_resource_contacts.test",
				ImportState:                          true,
				ImportStateVerify:                    true,
				ImportStateVerifyIdentifierAttribute: "resource_mrn",
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return s.RootModule().Resources["mondoo_resource_contacts.test"].Primary.Attributes["resource_mrn"], nil
				},
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

func testAccResourceContactsWithLinksConfig(orgID, spaceName string) string {
	return fmt.Sprintf(`
resource "mondoo_space" "test" {
  org_id = %[1]q
  name   = %[2]q
}

resource "mondoo_resource_contacts" "test" {
  resource_mrn = mondoo_space.test.mrn
  contacts     = ["charlie@example.com"]
  links = [
    { name = "Runbook", url = "https://wiki.example.com/runbook" },
    { name = "Dashboard", url = "https://grafana.example.com/d/prod" },
  ]
}
`, orgID, spaceName)
}

func testLinksList(t *testing.T, links ...[2]string) types.List {
	t.Helper()
	elements := make([]attr.Value, 0, len(links))
	for _, l := range links {
		elements = append(elements, types.ObjectValueMust(contactLinkObjectType.AttrTypes, map[string]attr.Value{
			"name": types.StringValue(l[0]),
			"url":  types.StringValue(l[1]),
		}))
	}
	return types.ListValueMust(contactLinkObjectType, elements)
}

func testLinkPayload(name, url string) ResourceContactPayload {
	return ResourceContactPayload{ContactType: ResourceContactTypeLink, Identity: mondoov1.String(url), Name: mondoov1.String(name)}
}

func TestSplitContactLinks(t *testing.T) {
	email := ResourceContactPayload{ContactType: mondoov1.ResourceContactTypeEmail, Identity: "a@example.com"}
	link := testLinkPayload("Runbook", "https://wiki.example.com")

	identities, links := splitContactLinks([]ResourceContactPayload{email, link})
	assert.Equal(t, []ResourceContactPayload{email}, identities)
	assert.Equal(t, []ResourceContactPayload{link}, links)
}

func TestReconcileLinks(t *testing.T) {
	runbook := [2]string{"Runbook", "https://wiki.example.com/runbook"}
	dash := [2]string{"Dashboard", "https://grafana.example.com/d/prod"}

	t.Run("unset links stay null when the server has none", func(t *testing.T) {
		got := reconcileLinks(types.ListNull(contactLinkObjectType), nil)
		assert.True(t, got.IsNull())
	})

	t.Run("keeps configured order regardless of server order", func(t *testing.T) {
		got := reconcileLinks(testLinksList(t, runbook, dash), []ResourceContactPayload{
			testLinkPayload(dash[0], dash[1]),
			testLinkPayload(runbook[0], runbook[1]),
		})
		assert.Equal(t, testLinksList(t, runbook, dash), got)
	})

	t.Run("drops links removed on the server and appends ones added outside Terraform", func(t *testing.T) {
		got := reconcileLinks(testLinksList(t, runbook), []ResourceContactPayload{
			testLinkPayload(dash[0], dash[1]),
		})
		assert.Equal(t, testLinksList(t, dash), got)
	})

	t.Run("a renamed link shows as drift", func(t *testing.T) {
		got := reconcileLinks(testLinksList(t, runbook), []ResourceContactPayload{
			testLinkPayload("Old name", runbook[1]),
		})
		assert.Equal(t, testLinksList(t, [2]string{"Old name", runbook[1]}), got)
	})
}

func TestExpandContactsSendsLinkNames(t *testing.T) {
	data := ResourceContactsResourceModel{
		Contacts: types.ListValueMust(types.StringType, []attr.Value{types.StringValue("a@example.com")}),
		Links:    testLinksList(t, [2]string{"Runbook", "https://wiki.example.com"}),
	}

	contacts, diags := expandContacts(context.Background(), data)
	require.False(t, diags.HasError())

	b, err := json.Marshal(contacts)
	require.NoError(t, err)
	// `name` is omitted for non-link contacts so older servers still accept them.
	assert.JSONEq(t, `[{"identity":"a@example.com"},{"identity":"https://wiki.example.com","name":"Runbook"}]`, string(b))
}
