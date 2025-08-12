// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"strings"

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
var _ resource.Resource = &S3BucketExportResource{}

func NewExportS3BucketResource() resource.Resource {
	return &S3BucketExportResource{}
}

type S3BucketExportResource struct {
	client *ExtendedGqlClient
}

type S3BucketExportResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn          types.String `tfsdk:"mrn"`
	Name         types.String `tfsdk:"name"`
	Bucket       types.String `tfsdk:"bucket_name"`
	Region       types.String `tfsdk:"region"`
	ExportFormat types.String `tfsdk:"export_format"`

	// credentials
	Credentials s3BucketExportCredentialModel `tfsdk:"credentials"`
}

type s3BucketExportCredentialModel struct {
	Key s3BucketExportKeyModel `tfsdk:"key"`
}

type s3BucketExportKeyModel struct {
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
}

func (r *S3BucketExportResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_export_s3"
}

func (r *S3BucketExportResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Export data to an Amazon S3 bucket.
			## Example Usage
			` + "```hcl" + `
			resource "mondoo_export_s3" "s3_export" {
				name        = "My S3 Export Integration"
				bucket_name = "my-mondoo-exports"
				region      = "us-west-2"
				export_format = "jsonl"
				
				credentials = {
					key = {
						access_key = var.aws_access_key
						secret_key = var.aws_secret_key
					}
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
				MarkdownDescription: "Name of the Amazon S3 bucket to export data to.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "AWS region where the S3 bucket is located.",
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
			},
			"credentials": schema.SingleNestedAttribute{
				MarkdownDescription: "Credentials for the Amazon S3 bucket.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"key": schema.SingleNestedAttribute{
						MarkdownDescription: "AWS access key credentials.",
						Required:            true,
						Attributes: map[string]schema.Attribute{
							"access_key": schema.StringAttribute{
								MarkdownDescription: "AWS access key ID.",
								Required:            true,
								Sensitive:           true,
							},
							"secret_key": schema.StringAttribute{
								MarkdownDescription: "AWS secret access key.",
								Required:            true,
								Sensitive:           true,
							},
						},
					},
				},
			},
		},
	}
}

func (r *S3BucketExportResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *S3BucketExportResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data S3BucketExportResourceModel

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

	// Determine output format
	outputFormat := mondoov1.BucketOutputTypeJsonl
	if strings.ToLower(data.ExportFormat.ValueString()) == "csv" {
		outputFormat = mondoov1.BucketOutputTypeCsv
	}

	// Create the export integration using the client
	integration, err := r.client.CreateIntegration(ctx,
		space.MRN(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAwsS3,
		mondoov1.ClientIntegrationConfigurationInput{
			AwsS3ConfigurationOptions: &mondoov1.AwsS3ConfigurationOptionsInput{
				Output:          outputFormat,
				Bucket:          mondoov1.String(data.Bucket.ValueString()),
				Region:          mondoov1.String(data.Region.ValueString()),
				AccessKey:       mondoov1.String(data.Credentials.Key.AccessKey.ValueString()),
				SecretAccessKey: mondoov1.String(data.Credentials.Key.SecretKey.ValueString()),
			},
		})

	if err != nil {
		resp.Diagnostics.AddError("Error creating S3 bucket export integration", err.Error())
		return
	}

	_, err = r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunExport)
	if err != nil {
		resp.Diagnostics.
			AddWarning("Client Error",
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

func (r *S3BucketExportResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data S3BucketExportResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *S3BucketExportResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data S3BucketExportResourceModel

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

	// Determine output format
	outputFormat := mondoov1.BucketOutputTypeJsonl
	if strings.ToLower(data.ExportFormat.ValueString()) == "csv" {
		outputFormat = mondoov1.BucketOutputTypeCsv
	}

	// Update the integration using the client
	_, err = r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAwsS3,
		mondoov1.ClientIntegrationConfigurationInput{
			AwsS3ConfigurationOptions: &mondoov1.AwsS3ConfigurationOptionsInput{
				Output:          outputFormat,
				Bucket:          mondoov1.String(data.Bucket.ValueString()),
				Region:          mondoov1.String(data.Region.ValueString()),
				AccessKey:       mondoov1.String(data.Credentials.Key.AccessKey.ValueString()),
				SecretAccessKey: mondoov1.String(data.Credentials.Key.SecretKey.ValueString()),
			},
		})

	if err != nil {
		resp.Diagnostics.AddError("Error updating S3 bucket export integration", err.Error())
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *S3BucketExportResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data S3BucketExportResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the integration using the client
	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error deleting S3 bucket export integration", err.Error())
		return
	}
}

func (r *S3BucketExportResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	model := S3BucketExportResourceModel{
		Mrn:          types.StringValue(integration.Mrn),
		Name:         types.StringValue(integration.Name),
		SpaceID:      types.StringValue(integration.SpaceID()),
		Bucket:       types.StringValue(integration.ConfigurationOptions.AwsS3ConfigurationOptions.Bucket),
		Region:       types.StringValue(integration.ConfigurationOptions.AwsS3ConfigurationOptions.Region),
		ExportFormat: types.StringValue(integration.ConfigurationOptions.AwsS3ConfigurationOptions.Output),

		Credentials: s3BucketExportCredentialModel{
			Key: s3BucketExportKeyModel{
				AccessKey: types.StringPointerValue(nil),
				SecretKey: types.StringPointerValue(nil),
			},
		},
	}

	resp.State.Set(ctx, &model)
}
