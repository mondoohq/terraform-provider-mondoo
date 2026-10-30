// Copyright Mondoo, Inc. 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	mondoov1 "go.mondoo.com/mondoo-go"
)

const testIntegrationMrn = "//captain.api.mondoo.app/spaces/hungry-poet-123456/integrations/2Abd08lk860"

func integrationErrorNotificationSchema(t *testing.T) resource.SchemaResponse {
	t.Helper()
	var resp resource.SchemaResponse
	(&IntegrationErrorNotificationResource{}).Schema(context.Background(), resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "schema diagnostics: %v", resp.Diagnostics)
	return resp
}

func TestIntegrationErrorNotificationSchemaIsValid(t *testing.T) {
	resp := integrationErrorNotificationSchema(t)
	diags := resp.Schema.ValidateImplementation(context.Background())
	assert.False(t, diags.HasError(), "schema implementation diagnostics: %v", diags)
	assert.True(t, resp.Schema.Attributes["target"].IsSensitive())
}

// The documents must match the API's GraphQL schema, so they are pinned here.

func TestGetIntegrationErrorNotificationRequest(t *testing.T) {
	c, got := newGraphqlTestClient(t, `{"data":{"integrationErrorNotification":{
		"integrationMrn":"`+testIntegrationMrn+`","workflowMrn":"//wf","channel":"SLACK","enabled":true}}}`)

	n, err := c.GetIntegrationErrorNotification(context.Background(), testIntegrationMrn)
	require.NoError(t, err)

	assert.Equal(t,
		`query($integrationMrn:ID!){integrationErrorNotification(integrationMrn: $integrationMrn){integrationMrn,workflowMrn,channel,enabled}}`,
		got.Query)
	assert.JSONEq(t, `"`+testIntegrationMrn+`"`, string(got.Variables["integrationMrn"]))
	assert.Equal(t, &IntegrationErrorNotificationPayload{IntegrationMrn: testIntegrationMrn, WorkflowMrn: "//wf", Channel: "SLACK", Enabled: true}, n)
}

func TestSetIntegrationErrorNotificationRequest(t *testing.T) {
	c, got := newGraphqlTestClient(t, `{"data":{"setIntegrationErrorNotification":{
		"integrationMrn":"`+testIntegrationMrn+`","workflowMrn":"//wf","channel":"EMAIL","enabled":true}}}`)

	n, err := c.SetIntegrationErrorNotification(context.Background(), mondoov1.SetIntegrationErrorNotificationInput{
		IntegrationMrn: mondoov1.ID(testIntegrationMrn),
		Channel:        mondoov1.IntegrationErrorNotificationChannelEmail,
		Target:         mondoov1.String("ops@example.com"),
	})
	require.NoError(t, err)

	assert.Equal(t,
		`mutation($input:SetIntegrationErrorNotificationInput!){setIntegrationErrorNotification(input: $input){integrationMrn,workflowMrn,channel,enabled}}`,
		got.Query)
	assert.JSONEq(t, `{"integrationMrn":"`+testIntegrationMrn+`","channel":"EMAIL","target":"ops@example.com"}`, string(got.Variables["input"]))
	assert.Equal(t, "//wf", n.WorkflowMrn)
}

func TestDeleteIntegrationErrorNotificationRequest(t *testing.T) {
	c, got := newGraphqlTestClient(t, `{"data":{"deleteIntegrationErrorNotification":true}}`)

	require.NoError(t, c.DeleteIntegrationErrorNotification(context.Background(), testIntegrationMrn))
	assert.Equal(t,
		`mutation($integrationMrn:ID!){deleteIntegrationErrorNotification(integrationMrn: $integrationMrn)}`,
		got.Query)
	assert.JSONEq(t, `"`+testIntegrationMrn+`"`, string(got.Variables["integrationMrn"]))
}

// readIntegrationErrorNotification runs Read on a stored alert against an API
// that answers with response, and returns the resulting state.
func readIntegrationErrorNotification(t *testing.T, response string) tfsdk.State {
	t.Helper()
	c, _ := newGraphqlTestClient(t, response)
	sch := integrationErrorNotificationSchema(t).Schema

	stored := tfsdk.State{Schema: sch, Raw: tftypes.NewValue(sch.Type().TerraformType(context.Background()), map[string]tftypes.Value{
		"integration_mrn": tftypes.NewValue(tftypes.String, testIntegrationMrn),
		"channel":         tftypes.NewValue(tftypes.String, "EMAIL"),
		"target":          tftypes.NewValue(tftypes.String, "ops@example.com"),
		"workflow_mrn":    tftypes.NewValue(tftypes.String, "//old"),
	})}

	resp := resource.ReadResponse{State: stored}
	(&IntegrationErrorNotificationResource{client: c}).Read(context.Background(), resource.ReadRequest{State: stored}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	return resp.State
}

func TestIntegrationErrorNotificationReadKeepsTarget(t *testing.T) {
	state := readIntegrationErrorNotification(t, `{"data":{"integrationErrorNotification":{
		"integrationMrn":"`+testIntegrationMrn+`","workflowMrn":"//new","channel":"SLACK","enabled":true}}}`)

	var got IntegrationErrorNotificationResourceModel
	require.False(t, state.Get(context.Background(), &got).HasError())
	assert.Equal(t, "SLACK", got.Channel.ValueString())
	assert.Equal(t, "//new", got.WorkflowMrn.ValueString())
	assert.Equal(t, "ops@example.com", got.Target.ValueString())
}

// An alert turned off outside of Terraform, or whose integration is gone,
// drops out of state so the next plan creates it again.
func TestIntegrationErrorNotificationReadDropsAlertThatIsOff(t *testing.T) {
	for name, response := range map[string]string{
		"deleted":  `{"data":{"integrationErrorNotification":null}}`,
		"disabled": `{"data":{"integrationErrorNotification":{"integrationMrn":"` + testIntegrationMrn + `","workflowMrn":"//wf","channel":"EMAIL","enabled":false}}}`,
		"integration gone": `{"errors":[{"message":"rpc error: code = NotFound desc = integration not found"}],
			"data":{"integrationErrorNotification":null}}`,
	} {
		t.Run(name, func(t *testing.T) {
			state := readIntegrationErrorNotification(t, response)
			assert.True(t, state.Raw.IsNull(), "expected the resource to be removed from state")
		})
	}
}
