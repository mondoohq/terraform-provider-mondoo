// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ExportBigQueryResource{}
var _ resource.ResourceWithConfigValidators = &ExportBigQueryResource{}

func NewMondooExportBigQueryResource() resource.Resource {
	return &ExportBigQueryResource{}
}

type ExportBigQueryResource struct {
	client *ExtendedGqlClient
}

type BigQueryExportResourceModel struct {
	// scope
	SpaceID  types.String `tfsdk:"space_id"`
	ScopeMrn types.String `tfsdk:"scope_mrn"`

	// integration details
	Mrn        types.String `tfsdk:"mrn"`
	Name       types.String `tfsdk:"name"`
	DatasetID  types.String `tfsdk:"dataset_id"`
	WifSubject types.String `tfsdk:"wif_subject"`

	// credentials
	ServiceAccountKey types.String                      `tfsdk:"service_account_key"`
	Credentials       *exportBigQueryCredentialsWrapper `tfsdk:"credentials"`
}

type exportBigQueryCredentialsWrapper struct {
	Wif *gcpWifCredentialModel `tfsdk:"wif"`
}

func (m BigQueryExportResourceModel) GetConfigurationOptions() *mondoov1.BigqueryConfigurationOptionsInput {
	opts := &mondoov1.BigqueryConfigurationOptionsInput{
		DatasetId: mondoov1.String(m.DatasetID.ValueString()),
	}

	if !m.ServiceAccountKey.IsNull() && !m.ServiceAccountKey.IsUnknown() {
		opts.ServiceAccount = mondoov1.NewStringPtr(mondoov1.String(m.ServiceAccountKey.ValueString()))
	}

	if m.Credentials != nil && m.Credentials.Wif != nil {
		opts.WifAudience = mondoov1.NewStringPtr(mondoov1.String(m.Credentials.Wif.Audience.ValueString()))
		if !m.Credentials.Wif.ServiceAccountEmail.IsNull() && !m.Credentials.Wif.ServiceAccountEmail.IsUnknown() {
			opts.WifServiceAccountEmail = mondoov1.NewStringPtr(mondoov1.String(m.Credentials.Wif.ServiceAccountEmail.ValueString()))
		}
	}

	return opts
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
				scope_mrn           = "//captain.api.mondoo.app/spaces/your-space-id"
				dataset_id          = "project-id.dataset_id"
				service_account_key = file("service-account.json")
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
				MarkdownDescription: "Google service account JSON key content. Mutually exclusive with `credentials.wif`.",
				Optional:            true,
				Sensitive:           true,
			},
			"credentials": schema.SingleNestedAttribute{
				MarkdownDescription: "Credentials for the BigQuery export. Provide `wif` for workload identity federation instead of the top-level `service_account_key`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"wif": schema.SingleNestedAttribute{
						MarkdownDescription: "Workload identity federation configuration. Mutually exclusive with `service_account_key`.",
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
					},
				},
			},
			"wif_subject": schema.StringAttribute{
				MarkdownDescription: "Computed OIDC subject used when Mondoo requests a WIF token for this integration. Configure your cloud provider's trust policy to accept this subject.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *ExportBigQueryResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("service_account_key"),
			path.MatchRoot("credentials").AtName("wif"),
		),
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

	configOpts := mondoov1.ClientIntegrationConfigurationInput{
		BigqueryConfigurationOptions: data.GetConfigurationOptions(),
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
			mondoov1.ClientIntegrationTypeBigquery,
			configOpts)
		if err != nil {
			resp.Diagnostics.AddError("Error creating BigQuery export integration", err.Error())
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
			mondoov1.ClientIntegrationTypeBigquery,
			configOpts)
		if err != nil {
			resp.Diagnostics.AddError("Error creating BigQuery export integration", err.Error())
			return
		}

		data.SpaceID = types.StringValue(space.ID())
		data.ScopeMrn = types.StringValue(space.MRN())
	}

	// Trigger export if enabled
	_, err := r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunExport)
	if err != nil {
		resp.Diagnostics.AddWarning("Client Warning",
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
		data.WifSubject = types.StringValue(fetched.ConfigurationOptions.BigqueryConfigurationOptions.WifSubject)
	}

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
	opts := integration.ConfigurationOptions.BigqueryConfigurationOptions
	data.Name = types.StringValue(integration.Name)
	data.WifSubject = types.StringValue(opts.WifSubject)
	if data.Credentials != nil && data.Credentials.Wif != nil {
		data.Credentials.Wif.Audience = types.StringValue(opts.WifAudience)
		data.Credentials.Wif.ServiceAccountEmail = stringOrNull(opts.WifServiceAccountEmail)
	}
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

	// Update the integration using the client
	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeBigquery,
		mondoov1.ClientIntegrationConfigurationInput{
			BigqueryConfigurationOptions: data.GetConfigurationOptions(),
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

	opts := integration.ConfigurationOptions.BigqueryConfigurationOptions
	model := BigQueryExportResourceModel{
		Mrn:               types.StringValue(integration.Mrn),
		Name:              types.StringValue(integration.Name),
		ScopeMrn:          types.StringValue(integration.ScopeMRN()),
		DatasetID:         types.StringValue(opts.DatasetId),
		WifSubject:        types.StringValue(opts.WifSubject),
		ServiceAccountKey: types.StringPointerValue(nil), // Don't expose sensitive data
	}
	if opts.WifAudience != "" {
		model.Credentials = &exportBigQueryCredentialsWrapper{
			Wif: &gcpWifCredentialModel{
				Audience:            types.StringValue(opts.WifAudience),
				ServiceAccountEmail: stringOrNull(opts.WifServiceAccountEmail),
			},
		}
	}

	if integration.IsSpaceScoped() {
		model.SpaceID = types.StringValue(integration.SpaceID())
	}

	resp.State.Set(ctx, &model)
}
