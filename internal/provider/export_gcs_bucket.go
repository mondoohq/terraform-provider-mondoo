// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ExportGcsBucketResource{}
var _ resource.ResourceWithConfigValidators = &ExportGcsBucketResource{}

func NewMondooExportGSCBucketResource() resource.Resource {
	return &ExportGcsBucketResource{}
}

type ExportGcsBucketResource struct {
	client *ExtendedGqlClient
}

type ExportGcsBucketResourceModel struct {
	// scope
	SpaceID  types.String `tfsdk:"space_id"`
	ScopeMrn types.String `tfsdk:"scope_mrn"`

	// integration details
	Mrn          types.String `tfsdk:"mrn"`
	Name         types.String `tfsdk:"name"`
	BucketName   types.String `tfsdk:"bucket_name"`
	ExportFormat types.String `tfsdk:"export_format"`
	WifSubject   types.String `tfsdk:"wif_subject"`

	// credentials
	Credential exportGcsBucketCredentialModel `tfsdk:"credentials"`
}

type exportGcsBucketCredentialModel struct {
	PrivateKey types.String           `tfsdk:"private_key"`
	Wif        *gcpWifCredentialModel `tfsdk:"wif"`
}

func (m ExportGcsBucketResourceModel) GetConfigurationOptions() *mondoov1.GcsBucketConfigurationOptionsInput {
	outputFormat := mondoov1.BucketOutputTypeJsonl
	if strings.ToLower(m.ExportFormat.ValueString()) == "csv" {
		outputFormat = mondoov1.BucketOutputTypeCsv
	}

	opts := &mondoov1.GcsBucketConfigurationOptionsInput{
		Output: outputFormat,
		Bucket: mondoov1.String(m.BucketName.ValueString()),
	}

	if !m.Credential.PrivateKey.IsNull() && !m.Credential.PrivateKey.IsUnknown() {
		opts.ServiceAccount = mondoov1.NewStringPtr(mondoov1.String(m.Credential.PrivateKey.ValueString()))
	}

	if m.Credential.Wif != nil {
		opts.WifAudience = mondoov1.NewStringPtr(mondoov1.String(m.Credential.Wif.Audience.ValueString()))
		if !m.Credential.Wif.ServiceAccountEmail.IsNull() && !m.Credential.Wif.ServiceAccountEmail.IsUnknown() {
			opts.WifServiceAccountEmail = mondoov1.NewStringPtr(mondoov1.String(m.Credential.Wif.ServiceAccountEmail.ValueString()))
		}
	}

	return opts
}

func (r *ExportGcsBucketResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_export_gcs_bucket"
}

func (r *ExportGcsBucketResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Export data to a Google Cloud Storage bucket.
			## Example Usage
			` + "```hcl" + `
			resource "mondoo_export_gcs_bucket" "test" {
				name          = "bucket-export-integration"
				bucket_name   = "my-bucket-name"
				scope_mrn     = "//captain.api.mondoo.app/spaces/your-space-id"
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
				DeprecationMessage:  "Use `scope_mrn` instead. This attribute will be removed in a future version.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("scope_mrn")),
				},
			},
			"scope_mrn": schema.StringAttribute{
				MarkdownDescription: "The MRN of the scope (space, organization, or platform) for the export integration.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("space_id")),
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
			"wif_subject": schema.StringAttribute{
				MarkdownDescription: "Computed OIDC subject used when Mondoo requests a WIF token for this integration. Configure your cloud provider's trust policy to accept this subject.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"credentials": schema.SingleNestedAttribute{
				MarkdownDescription: "Credentials for the Google Cloud Storage bucket. Provide either a static service account `private_key` or a `wif` block for workload identity federation.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"private_key": schema.StringAttribute{
						MarkdownDescription: "Private key for the service account in JSON format. Mutually exclusive with `wif`.",
						Optional:            true,
						Sensitive:           true,
						Validators: []validator.String{
							stringvalidator.ConflictsWith(
								path.MatchRoot("credentials").AtName("wif"),
							),
						},
					},
					"wif": schema.SingleNestedAttribute{
						MarkdownDescription: "Workload identity federation configuration. Mutually exclusive with `private_key`.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"audience": schema.StringAttribute{
								MarkdownDescription: "WIF audience URL for GCP workload identity federation.",
								Required:            true,
							},
							"service_account_email": schema.StringAttribute{
								MarkdownDescription: "Optional GCP service account email to impersonate via workload identity federation.",
								Optional:            true,
							},
						},
						Validators: []validator.Object{
							objectvalidator.ConflictsWith(
								path.MatchRoot("credentials").AtName("private_key"),
							),
						},
					},
				},
			},
		},
	}
}

func (r *ExportGcsBucketResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot("credentials").AtName("private_key"),
			path.MatchRoot("credentials").AtName("wif"),
		),
	}
}

func (r *ExportGcsBucketResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ExportGcsBucketResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ExportGcsBucketResourceModel

	// Read the plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	configOpts := mondoov1.ClientIntegrationConfigurationInput{
		GcsBucketConfigurationOptions: data.GetConfigurationOptions(),
	}

	var integration *CreateClientIntegrationPayload
	if !data.ScopeMrn.IsNull() && data.ScopeMrn.ValueString() != "" {
		// New path: use scope_mrn directly
		scopeMrn := data.ScopeMrn.ValueString()
		ctx = tflog.SetField(ctx, "scope_mrn", scopeMrn)

		var err error
		integration, err = r.client.CreateScopedIntegration(ctx,
			scopeMrn,
			data.Name.ValueString(),
			mondoov1.ClientIntegrationTypeGcsBucket,
			configOpts)
		if err != nil {
			resp.Diagnostics.AddError("Error creating GCS bucket export integration", err.Error())
			return
		}

		data.SpaceID = types.StringNull()
		data.ScopeMrn = types.StringValue(scopeMrn)
	} else {
		// Legacy path: use space_id
		space, err := r.client.ComputeSpace(data.SpaceID)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Configuration", err.Error())
			return
		}
		ctx = tflog.SetField(ctx, "space_mrn", space.MRN())

		integration, err = r.client.CreateIntegration(ctx,
			space.MRN(),
			data.Name.ValueString(),
			mondoov1.ClientIntegrationTypeGcsBucket,
			configOpts)
		if err != nil {
			resp.Diagnostics.AddError("Error creating GCS bucket export integration", err.Error())
			return
		}

		data.SpaceID = types.StringValue(space.ID())
		data.ScopeMrn = types.StringValue(space.MRN())
	}

	_, err := r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunExport)
	if err != nil {
		resp.Diagnostics.
			AddWarning("Client Error",
				fmt.Sprintf("Unable to trigger export for integration. Got error: %s", err),
			)
	}

	// Save data into the Terraform state
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))

	// Fetch the full integration to populate server-computed fields (e.g. wif_subject)
	fetched, err := r.client.GetClientIntegration(ctx, string(integration.Mrn))
	if err != nil {
		resp.Diagnostics.AddWarning("Client Warning",
			fmt.Sprintf("Unable to fetch integration after create to populate computed fields. Got error: %s", err))
		data.WifSubject = types.StringNull()
	} else {
		data.WifSubject = types.StringValue(fetched.ConfigurationOptions.GcsBucketConfigurationOptions.WifSubject)
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ExportGcsBucketResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ExportGcsBucketResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Refresh server-computed fields (e.g. wif_subject) from the API.
	integration, err := r.client.GetClientIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading GCS bucket export integration", err.Error())
		return
	}
	opts := integration.ConfigurationOptions.GcsBucketConfigurationOptions
	data.WifSubject = types.StringValue(opts.WifSubject)
	if data.Credential.Wif != nil {
		data.Credential.Wif.Audience = types.StringValue(opts.WifAudience)
		data.Credential.Wif.ServiceAccountEmail = stringOrNull(opts.WifServiceAccountEmail)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ExportGcsBucketResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ExportGcsBucketResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeGcsBucket,
		mondoov1.ClientIntegrationConfigurationInput{
			GcsBucketConfigurationOptions: data.GetConfigurationOptions(),
		})

	if err != nil {
		resp.Diagnostics.AddError("Error updating GCS bucket export integration", err.Error())
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ExportGcsBucketResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ExportGcsBucketResourceModel

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

func (r *ExportGcsBucketResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	opts := integration.ConfigurationOptions.GcsBucketConfigurationOptions
	model := ExportGcsBucketResourceModel{
		Mrn:          types.StringValue(integration.Mrn),
		Name:         types.StringValue(integration.Name),
		ScopeMrn:     types.StringValue(integration.ScopeMRN()),
		BucketName:   types.StringValue(opts.Bucket),
		ExportFormat: types.StringValue(opts.Output),
		WifSubject:   types.StringValue(opts.WifSubject),

		Credential: exportGcsBucketCredentialModel{
			PrivateKey: types.StringPointerValue(nil),
		},
	}
	if opts.WifAudience != "" {
		model.Credential.Wif = &gcpWifCredentialModel{
			Audience:            types.StringValue(opts.WifAudience),
			ServiceAccountEmail: stringOrNull(opts.WifServiceAccountEmail),
		}
	}

	if integration.IsSpaceScoped() {
		model.SpaceID = types.StringValue(integration.SpaceID())
	}

	resp.State.Set(ctx, &model)
}
