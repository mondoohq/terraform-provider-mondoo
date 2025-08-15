// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &GcsBucketExportResource{}

func NewExportGSCBucketResource() resource.Resource {
	return &GcsBucketExportResource{}
}

type GcsBucketExportResource struct {
	client *ExtendedGqlClient
}

type GcsBucketExportResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn          types.String `tfsdk:"mrn"`
	Name         types.String `tfsdk:"name"`
	BucketName   types.String `tfsdk:"bucket_name"`
	ExportFormat types.String `tfsdk:"export_format"`

	// credentials
	Credential gcsBucketExportCredentialModel `tfsdk:"credentials"`
}

type gcsBucketExportCredentialModel struct {
	PrivateKey types.String `tfsdk:"private_key"`
}

func (r *GcsBucketExportResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gcs_bucket_export"
}

func (r *GcsBucketExportResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		DeprecationMessage: "Use `mondoo_export_gcs_bucket` instead.",
		MarkdownDescription: `Export data to a Google Cloud Storage bucket.
			## Example Usage
			` + "```hcl" + `
			resource "mondoo_gcs_bucket_export" "test" {
				name         = "bucket-export-integration"
				bucket_name  = "my-bucket-name"
				space_id     = "your-space-id"
				export_format = "jsonl"
				  credentials = {
					private_key = base64decode(google_service_account_key.mondoo_integration.private_key)
				}
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
				MarkdownDescription: "Name of the export integration.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},

			"bucket_name": schema.StringAttribute{
				MarkdownDescription: "Name of the Google Cloud Storage bucket to export data to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"export_format": schema.StringAttribute{
				MarkdownDescription: "Format of the export (JSONL or CSV), defaults to JSONL.",
				Optional:            true,
				Default:             stringdefault.StaticString("jsonl"),
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"credentials": schema.SingleNestedAttribute{
				MarkdownDescription: "Credentials for the Google Cloud Storage bucket.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"private_key": schema.StringAttribute{
						MarkdownDescription: "Private key for the service account in JSON format.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
		},
	}
}

func (r *GcsBucketExportResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client. Got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *GcsBucketExportResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data GcsBucketExportResourceModel

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
		mondoov1.ClientIntegrationTypeGcsBucket,
		mondoov1.ClientIntegrationConfigurationInput{
			GcsBucketConfigurationOptions: &mondoov1.GcsBucketConfigurationOptionsInput{
				Output:         mondoov1.BucketOutputTypeJsonl,
				Bucket:         mondoov1.String(data.BucketName.ValueString()),
				ServiceAccount: mondoov1.String(data.Credential.PrivateKey.ValueString()),
			},
		})

	if err != nil {
		resp.Diagnostics.AddError("Error creating GCS bucket export integration", err.Error())
		return
	}
	_, err = r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunExport)
	if err != nil {
		resp.Diagnostics.
			AddWarning("Client Error",
				fmt.Sprintf("Unable to trigger export for integration. Got error: %s", err),
			)
		return
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))
	data.SpaceID = types.StringValue(space.ID())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GcsBucketExportResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data GcsBucketExportResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GcsBucketExportResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data GcsBucketExportResourceModel

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

	// Do GraphQL request to API to update the resource.
	_, err = r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeGcsBucket,
		mondoov1.ClientIntegrationConfigurationInput{
			GcsBucketConfigurationOptions: &mondoov1.GcsBucketConfigurationOptionsInput{
				Output:         mondoov1.BucketOutputTypeJsonl,
				Bucket:         mondoov1.String(data.BucketName.ValueString()),
				ServiceAccount: mondoov1.String(data.Credential.PrivateKey.ValueString()),
			},
		})

	if err != nil {
		resp.Diagnostics.AddError("Error updating GCS bucket export integration", err.Error())
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *GcsBucketExportResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data GcsBucketExportResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// Delete the integration using the client
	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting GCS bucket export integration", err.Error())
		return
	}
}

func (r *GcsBucketExportResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	model := GcsBucketExportResourceModel{
		Mrn:          types.StringValue(integration.Mrn),
		Name:         types.StringValue(integration.Name),
		SpaceID:      types.StringValue(integration.SpaceID()),
		BucketName:   types.StringValue(integration.ConfigurationOptions.GcsBucketConfigurationOptions.Bucket),
		ExportFormat: types.StringValue(integration.ConfigurationOptions.GcsBucketConfigurationOptions.Output),

		Credential: gcsBucketExportCredentialModel{
			PrivateKey: types.StringPointerValue(nil),
		},
	}

	resp.State.Set(ctx, &model)
}
