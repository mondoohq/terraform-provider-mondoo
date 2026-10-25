// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"testing"

	fwresource "github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mondoov1 "go.mondoo.com/mondoo-go"
)

func TestAccWorkspaceResource(t *testing.T) {
	// Generate random names for each resource instance to avoid state conflicts
	randName1 := fmt.Sprintf("test-ws-%d", rand.Intn(10000))
	randName1Updated := fmt.Sprintf("%s-updated", randName1)

	randName2 := fmt.Sprintf("test-ws-prov-%d", rand.Intn(10000))
	randName2Updated := fmt.Sprintf("%s-updated", randName2)

	var workspaceMrn string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Create and Read testing
			{
				Config: testAccWorkspaceResourceConfig(accSpace.ID(), randName1, "development"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "name", randName1),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "space_id", accSpace.ID()),
				),
			},
			// Update and Read testing
			{
				Config: testAccWorkspaceResourceConfig(accSpace.ID(), randName1Updated, "production"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "name", randName1Updated),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "space_id", accSpace.ID()),
					testAccCaptureWorkspaceMrn("mondoo_workspace.test", &workspaceMrn),
				),
			},
			// A workspace deleted outside of Terraform is created again
			{
				PreConfig: func() { testAccDeleteWorkspace(t, workspaceMrn) },
				Config:    testAccWorkspaceResourceConfig(accSpace.ID(), randName1Updated, "production"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("mondoo_workspace.test", plancheck.ResourceActionCreate),
					},
				},
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "name", randName1Updated),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "space_id", accSpace.ID()),
				),
			},
			// Create and Read testing
			{
				Config: testAccWorkspaceResourceWithSpaceInProviderConfig(accSpace.ID(), randName2, "qa"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "name", randName2),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "space_id", accSpace.ID()),
				),
			},
			// Update and Read testing
			{
				Config: testAccWorkspaceResourceWithSpaceInProviderConfig(accSpace.ID(), randName2Updated, "production"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("mondoo_workspace.test", "name", randName2Updated),
					resource.TestCheckResourceAttr("mondoo_workspace.test", "space_id", accSpace.ID()),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccWorkspaceResourceConfig(spaceID, name, env string) string {
	return fmt.Sprintf(`
resource "mondoo_workspace" "test" {
  space_id         = %[1]q
  name             = %[2]q
  asset_selections = [
    {
      conditions = [
        {
          operator = "AND"
          key_value_condition = {
            field    = "LABELS"
            operator = "CONTAINS"
            values = [
              {
                key   = "environment"
                value = %[3]q
              }
            ]
          }
        }
      ]
    }
  ]
}
`, spaceID, name, env)
}

func testAccWorkspaceResourceWithSpaceInProviderConfig(spaceID, name, env string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}

resource "mondoo_workspace" "test" {
  name             = %[2]q
  asset_selections = [
    {
      conditions = [
        {
          operator = "AND"
          key_value_condition = {
            field    = "LABELS"
            operator = "CONTAINS"
            values = [
              {
                key   = "environment"
                value = %[3]q
              }
            ]
          }
        }
      ]
    }
  ]
}
`, spaceID, name, env)
}

// testAccCaptureWorkspaceMrn stores the workspace's MRN so a later step can
// change the workspace outside of Terraform.
func testAccCaptureWorkspaceMrn(name string, mrn *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("resource %s not found in state", name)
		}
		*mrn = rs.Primary.Attributes["mrn"]
		if *mrn == "" {
			return fmt.Errorf("resource %s has no mrn", name)
		}
		return nil
	}
}

// testAccDeleteWorkspace deletes a workspace through the API, as deleting it
// in the console would.
func testAccDeleteWorkspace(t *testing.T, mrn string) {
	t.Helper()
	client, err := mondooClient()
	require.NoError(t, err)

	var deleteMutation struct {
		DeleteWorkspace mondoov1.Boolean `graphql:"deleteWorkspaces(input: $input)"`
	}
	input := mondoov1.DeleteWorkspacesInput{
		Mrns: []mondoov1.String{mondoov1.String(mrn)},
	}
	require.NoError(t, client.Mutate(context.Background(), &deleteMutation, input, nil))
}

const testWorkspaceMrn = "//captain.api.mondoo.app/spaces/my-space-123456/workspaces/my-workspace"

func TestQueryWorkspace(t *testing.T) {
	c, got := newGraphqlTestClient(t, `{"data":{"workspace":{
		"__typename":"Workspace",
		"mrn":"`+testWorkspaceMrn+`",
		"ownerMrn":"//captain.api.mondoo.app/spaces/my-space-123456",
		"name":"my workspace",
		"description":"desc",
		"selections":{"selections":[]}}}}`)

	m, err := (&WorkspaceResource{client: c}).queryWorkspace(context.Background(), testWorkspaceMrn)
	require.NoError(t, err)

	assert.Contains(t, got.Query, "__typename")
	assert.Contains(t, got.Query, "... on NotFoundError")
	assert.Contains(t, got.Query, "... on RequestError")
	assert.JSONEq(t, `"`+testWorkspaceMrn+`"`, string(got.Variables["mrn"]))

	assert.Equal(t, testWorkspaceMrn, m.Mrn.ValueString())
	assert.Equal(t, "my-space-123456", m.SpaceID.ValueString())
	assert.Equal(t, "my workspace", m.Name.ValueString())
	assert.Equal(t, "desc", m.Description.ValueString())
}

func TestQueryWorkspaceNotFound(t *testing.T) {
	c, _ := newGraphqlTestClient(t, `{"data":{"workspace":{
		"__typename":"NotFoundError","message":"workspace not found"}}}`)

	_, err := (&WorkspaceResource{client: c}).queryWorkspace(context.Background(), testWorkspaceMrn)
	assert.True(t, errors.Is(err, errWorkspaceNotFound), "got %v", err)
}

func TestQueryWorkspaceRequestError(t *testing.T) {
	c, _ := newGraphqlTestClient(t, `{"data":{"workspace":{
		"__typename":"RequestError","message":"permission denied"}}}`)

	_, err := (&WorkspaceResource{client: c}).queryWorkspace(context.Background(), testWorkspaceMrn)
	require.Error(t, err)
	assert.False(t, errors.Is(err, errWorkspaceNotFound))
	assert.Contains(t, err.Error(), "permission denied")
}

// A workspace deleted outside of Terraform is removed from state, so the next
// plan creates it instead of updating it with empty values.
func TestWorkspaceReadRemovesDeletedWorkspace(t *testing.T) {
	c, _ := newGraphqlTestClient(t, `{"data":{"workspace":{
		"__typename":"NotFoundError","message":"workspace not found"}}}`)
	r := &WorkspaceResource{client: c}

	var schemaResp fwresource.SchemaResponse
	r.Schema(context.Background(), fwresource.SchemaRequest{}, &schemaResp)
	require.False(t, schemaResp.Diagnostics.HasError())

	state := tfsdk.State{Schema: schemaResp.Schema}
	diags := state.Set(context.Background(), &WorkspaceResourceModel{
		SpaceID:     types.StringValue("my-space-123456"),
		Mrn:         types.StringValue(testWorkspaceMrn),
		Name:        types.StringValue("my workspace"),
		Description: types.StringValue(""),
		Selections:  []WorkspaceSelectionModel{},
	})
	require.False(t, diags.HasError(), "set state: %v", diags)

	resp := fwresource.ReadResponse{State: state}
	r.Read(context.Background(), fwresource.ReadRequest{State: state}, &resp)

	require.False(t, resp.Diagnostics.HasError(), "read diagnostics: %v", resp.Diagnostics)
	assert.True(t, resp.State.Raw.IsNull(), "workspace should be removed from state")
}
