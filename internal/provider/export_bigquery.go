// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ExportBigQueryResource{}

func NewMondooExportBigQueryResource() resource.Resource {
	return &ExportBigQueryResource{}
}

type ExportBigQueryResource struct {
	client *ExtendedGqlClient
}

type BigQueryExportResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn       types.String `tfsdk:"mrn"`
	Name      types.String `tfsdk:"name"`
	DatasetID types.String `tfsdk:"dataset_id"`

	// credentials
	ServiceAccountKey types.String `tfsdk:"service_account_key"`
}

func (r *ExportBigQueryResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_export_bigquery"
}

func (r *ExportBigQueryResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Export data to Google BigQuery.
			## Example Usage
			` + "```hcl" + `
			resource "mondoo_export_bigquery" "example" {
				name                = "enterprise-demo-BigQuery"
				dataset_id          = "project-id.dataset_id"
				service_account_key = file("service-account.json")
			}
			` + "```" + `
		`,
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo space identifier. If there is no space ID, the provider space is used.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mrn": schema.StringAttribute{
				MarkdownDescription: "Mondoo resource name (MRN) of the integration.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "A descriptive name for the integration.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"dataset_id": schema.StringAttribute{
				MarkdownDescription: "Target BigQuery dataset (project-id.dataset_id).",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"service_account_key": schema.StringAttribute{
				MarkdownDescription: "Google service account JSON key content.",
				Required:            true,
				Sensitive:           true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"schedule": schema.StringAttribute{
				MarkdownDescription: "Frequency of export (e.g., hourly, daily). Defaults to hourly.",
				Optional:            true,
				Default:             stringdefault.StaticString("hourly"),
				Computed:            true,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Whether the export is active. Defaults to true.",
				Optional:            true,
				Default:             booldefault.StaticBool(true),
				Computed:            true,
			},
		},
	}
}

func (r *ExportBigQueryResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ExtendedGqlClient. Got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *ExportBigQueryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data BigQueryExportResourceModel

	// Read the plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Compute and validate the space
	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "space_mrn", space.MRN())

	// Create the export integration using the client
	integration, err := r.client.CreateIntegration(ctx,
		space.MRN(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeBigquery,
		mondoov1.ClientIntegrationConfigurationInput{
			BigqueryConfigurationOptions: &mondoov1.BigqueryConfigurationOptionsInput{
				DatasetId:      mondoov1.String(data.DatasetID.ValueString()),
				ServiceAccount: mondoov1.String(data.ServiceAccountKey.ValueString()),
			},
		})

	if err != nil {
		resp.Diagnostics.AddError("Error creating BigQuery export integration", err.Error())
		return
	}

	// Trigger export if enabled
	_, err = r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunExport)
	if err != nil {
		resp.Diagnostics.AddWarning("Client Warning",
			fmt.Sprintf("Unable to trigger export for integration. Got error: %s", err),
		)
	}

	// Save data into the Terraform state
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))
	data.SpaceID = types.StringValue(space.ID())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ExportBigQueryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data BigQueryExportResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get integration details from the API
	integration, err := r.client.GetClientIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading BigQuery export integration", err.Error())
		return
	}

	// Update the state with the latest information
	data.Name = types.StringValue(integration.Name)
	// Note: We don't update service_account_key to avoid showing sensitive data

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ExportBigQueryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data BigQueryExportResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Compute and validate the space
	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "space_mrn", space.MRN())

	// Update the integration using the client
	_, err = r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeBigquery,
		mondoov1.ClientIntegrationConfigurationInput{
			BigqueryConfigurationOptions: &mondoov1.BigqueryConfigurationOptionsInput{
				DatasetId:      mondoov1.String(data.DatasetID.ValueString()),
				ServiceAccount: mondoov1.String(data.ServiceAccountKey.ValueString()),
			},
		})

	if err != nil {
		resp.Diagnostics.AddError("Error updating BigQuery export integration", err.Error())
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ExportBigQueryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data BigQueryExportResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the integration using the client
	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting BigQuery export integration", err.Error())
		return
	}
}

func (r *ExportBigQueryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	model := BigQueryExportResourceModel{
		Mrn:               types.StringValue(integration.Mrn),
		Name:              types.StringValue(integration.Name),
		SpaceID:           types.StringValue(integration.SpaceID()),
		DatasetID:         types.StringValue(integration.ConfigurationOptions.BigqueryConfigurationOptions.DatasetId),
		ServiceAccountKey: types.StringPointerValue(nil), // Don't expose sensitive data
	}

	resp.State.Set(ctx, &model)
}
