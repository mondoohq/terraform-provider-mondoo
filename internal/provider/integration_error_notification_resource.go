// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var (
	_ resource.Resource                = (*IntegrationErrorNotificationResource)(nil)
	_ resource.ResourceWithConfigure   = (*IntegrationErrorNotificationResource)(nil)
	_ resource.ResourceWithImportState = (*IntegrationErrorNotificationResource)(nil)
)

func NewIntegrationErrorNotificationResource() resource.Resource {
	return &IntegrationErrorNotificationResource{}
}

// IntegrationErrorNotificationResource manages an integration's failure
// alert. An integration has at most one, so the integration MRN is its ID.
type IntegrationErrorNotificationResource struct {
	client *ExtendedGqlClient
}

type IntegrationErrorNotificationResourceModel struct {
	IntegrationMrn types.String `tfsdk:"integration_mrn"`
	Channel        types.String `tfsdk:"channel"`
	Target         types.String `tfsdk:"target"`
	WorkflowMrn    types.String `tfsdk:"workflow_mrn"`
}

func (r *IntegrationErrorNotificationResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_error_notification"
}

func (r *IntegrationErrorNotificationResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Sends an alert when an integration moves to an error state, by email, Slack, Microsoft Teams, or webhook. Works with every integration type.

An integration has at most one alert. Creating this resource replaces any alert the integration already has, and destroying it turns the alert off.

The alert is a workflow, which you can see in the workflows view of the Mondoo console. Deleting or disabling that workflow in the console turns the alert off, and the next apply turns it back on.

Managing this resource requires permission to create and delete workflows in the integration's space.

~> **Note:** The API never returns the alert's ` + "`target`" + `, because a webhook URL works as a secret. Terraform detects a changed ` + "`channel`" + ` or an alert turned off outside of Terraform, but not a target changed in the console.`,

		Attributes: map[string]schema.Attribute{
			"integration_mrn": schema.StringAttribute{
				MarkdownDescription: "The Mondoo Resource Name (MRN) of the integration to alert on, such as `mondoo_integration_aws.example.mrn`.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"channel": schema.StringAttribute{
				MarkdownDescription: "Where to deliver the alert: `EMAIL`, `SLACK`, `TEAMS`, or `WEBHOOK`.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(mondoov1.IntegrationErrorNotificationChannelEmail),
						string(mondoov1.IntegrationErrorNotificationChannelSlack),
						string(mondoov1.IntegrationErrorNotificationChannelTeams),
						string(mondoov1.IntegrationErrorNotificationChannelWebhook),
					),
				},
			},
			"target": schema.StringAttribute{
				MarkdownDescription: "The email address (`EMAIL`), or the incoming-webhook URL (`SLACK`, `TEAMS`, `WEBHOOK`).",
				Required:            true,
				Sensitive:           true,
			},
			"workflow_mrn": schema.StringAttribute{
				MarkdownDescription: "The MRN of the workflow that sends the alert.",
				Computed:            true,
			},
		},
	}
}

func (r *IntegrationErrorNotificationResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ExtendedGqlClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}

func (r *IntegrationErrorNotificationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.set(ctx, req.Plan, &resp.State, &resp.Diagnostics, "create")
}

func (r *IntegrationErrorNotificationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.set(ctx, req.Plan, &resp.State, &resp.Diagnostics, "update")
}

// set writes the planned alert. The API replaces any alert the integration
// has, so create and update are the same call.
func (r *IntegrationErrorNotificationResource) set(ctx context.Context, plan tfsdk.Plan, state *tfsdk.State, diags *diag.Diagnostics, verb string) {
	var data IntegrationErrorNotificationResourceModel
	diags.Append(plan.Get(ctx, &data)...)
	if diags.HasError() {
		return
	}

	n, err := r.client.SetIntegrationErrorNotification(ctx, mondoov1.SetIntegrationErrorNotificationInput{
		IntegrationMrn: mondoov1.ID(data.IntegrationMrn.ValueString()),
		Channel:        mondoov1.IntegrationErrorNotificationChannel(data.Channel.ValueString()),
		Target:         mondoov1.String(data.Target.ValueString()),
	})
	if err != nil {
		diags.AddError(fmt.Sprintf("Failed to %s integration error notification", verb), err.Error())
		return
	}

	data.WorkflowMrn = types.StringValue(n.WorkflowMrn)
	diags.Append(state.Set(ctx, &data)...)
}

func (r *IntegrationErrorNotificationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IntegrationErrorNotificationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	n, err := r.client.GetIntegrationErrorNotification(ctx, data.IntegrationMrn.ValueString())
	if err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to read integration error notification", err.Error())
		return
	}
	// The integration is gone, or its alert was deleted or disabled outside
	// of Terraform: the next plan creates the alert again.
	if err != nil || n == nil || !n.Enabled {
		tflog.Warn(ctx, "integration error notification is off, removing from state", map[string]interface{}{
			"integrationMrn": data.IntegrationMrn.ValueString(),
		})
		resp.State.RemoveResource(ctx)
		return
	}

	// target is not returned by the API, so the state keeps the configured one.
	data.Channel = types.StringValue(n.Channel)
	data.WorkflowMrn = types.StringValue(n.WorkflowMrn)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IntegrationErrorNotificationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data IntegrationErrorNotificationResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// An integration already deleted took its alert with it.
	err := r.client.DeleteIntegrationErrorNotification(ctx, data.IntegrationMrn.ValueString())
	if err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to delete integration error notification", err.Error())
	}
}

// ImportState takes the integration MRN. The target is not returned by the
// API, so the first plan after an import sets it from the configuration.
func (r *IntegrationErrorNotificationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("integration_mrn"), req, resp)
}
