// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var _ resource.Resource = (*integrationAuditLogExportResource)(nil)

func NewIntegrationAuditLogExportResource() resource.Resource {
	return &integrationAuditLogExportResource{}
}

type integrationAuditLogExportResource struct {
	client *ExtendedGqlClient
}

type integrationAuditLogExportResourceModel struct {
	// scope
	OrgID    types.String `tfsdk:"org_id"`
	ScopeMrn types.String `tfsdk:"scope_mrn"`

	// integration details
	Mrn  types.String `tfsdk:"mrn"`
	Name types.String `tfsdk:"name"`

	// configuration
	Bucket            types.String `tfsdk:"bucket"`
	IncludeHistorical types.Bool   `tfsdk:"include_historical"`

	// credentials
	ServiceAccountJSON types.String `tfsdk:"service_account_json"`
	WifAudience        types.String `tfsdk:"wif_audience"`
	WifSAEmail         types.String `tfsdk:"wif_service_account_email"`
}

func (m integrationAuditLogExportResourceModel) GetConfigurationOptions() mondoov1.ClientIntegrationConfigurationInput {
	opts := &mondoov1.AuditLogExportConfigurationOptionsInput{
		Bucket: mondoov1.String(m.Bucket.ValueString()),
	}

	if !m.IncludeHistorical.IsNull() && !m.IncludeHistorical.IsUnknown() {
		v := mondoov1.Boolean(m.IncludeHistorical.ValueBool())
		opts.IncludeHistorical = &v
	}

	if sa := m.ServiceAccountJSON.ValueString(); sa != "" {
		opts.ServiceAccountJson = mondoov1.NewStringPtr(mondoov1.String(sa))
	}

	if aud := m.WifAudience.ValueString(); aud != "" {
		opts.WifAudience = mondoov1.NewStringPtr(mondoov1.String(aud))
	}

	if email := m.WifSAEmail.ValueString(); email != "" {
		opts.WifServiceAccountEmail = mondoov1.NewStringPtr(mondoov1.String(email))
	}

	return mondoov1.ClientIntegrationConfigurationInput{
		AuditLogExportConfigurationOptions: opts,
	}
}

func (r *integrationAuditLogExportResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_audit_log_export"
}

func (r *integrationAuditLogExportResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Export Mondoo audit logs to a GCS bucket in OCSF format for ingestion by third-party SIEM systems.",
		Attributes: map[string]schema.Attribute{
			"org_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo organization identifier. Use this for org-scoped integrations. Conflicts with `scope_mrn`.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"scope_mrn": schema.StringAttribute{
				MarkdownDescription: "Scope MRN for the integration. Use `//platform.api.mondoo.app` for platform-level exports. Conflicts with `org_id`.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mrn": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Integration identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the integration.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"bucket": schema.StringAttribute{
				MarkdownDescription: "GCS bucket name for audit log export.",
				Required:            true,
			},
			"include_historical": schema.BoolAttribute{
				MarkdownDescription: "Whether to include historical audit logs on first export. Default false.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"service_account_json": schema.StringAttribute{
				MarkdownDescription: "GCS service account JSON credentials. Either this or WIF credentials must be provided.",
				Optional:            true,
				Sensitive:           true,
			},
			"wif_audience": schema.StringAttribute{
				MarkdownDescription: "WIF audience URL for GCP workload identity federation.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
			"wif_service_account_email": schema.StringAttribute{
				MarkdownDescription: "GCP service account email for WIF service account impersonation.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(""),
			},
		},
	}
}

func (r *integrationAuditLogExportResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ExtendedGqlClient. Got: %T.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *integrationAuditLogExportResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationAuditLogExportResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	scopeMrn := data.ScopeMrn.ValueString()
	if scopeMrn == "" {
		if data.OrgID.ValueString() == "" {
			resp.Diagnostics.AddError("Configuration Error", "Either org_id or scope_mrn must be provided.")
			return
		}
		scopeMrn = orgPrefix + data.OrgID.ValueString()
	}
	ctx = tflog.SetField(ctx, "scope_mrn", scopeMrn)

	tflog.Debug(ctx, "Creating audit log export integration")
	integration, err := r.client.CreateIntegration(ctx,
		scopeMrn,
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAuditLogExport,
		data.GetConfigurationOptions(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to create audit log export integration: %s", err),
		)
		return
	}

	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAuditLogExportResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationAuditLogExportResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAuditLogExportResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationAuditLogExportResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAuditLogExport,
		data.GetConfigurationOptions(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to update audit log export integration: %s", err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAuditLogExportResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationAuditLogExportResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to delete audit log export integration: %s", err),
		)
		return
	}
}
