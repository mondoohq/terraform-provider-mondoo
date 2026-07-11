// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                   = (*integrationGcpServerlessResource)(nil)
	_ resource.ResourceWithImportState    = (*integrationGcpServerlessResource)(nil)
	_ resource.ResourceWithValidateConfig = (*integrationGcpServerlessResource)(nil)
)

func NewIntegrationGcpServerlessResource() resource.Resource {
	return &integrationGcpServerlessResource{}
}

type integrationGcpServerlessResource struct {
	client *ExtendedGqlClient
}

type integrationGcpServerlessResourceModel struct {
	// ScopeMrn is the MRN of the scope (space, organization, or platform) the
	// integration is created under. When omitted, the provider's configured
	// space is used. Required for org-scoped / cross-org integrations.
	ScopeMrn types.String `tfsdk:"scope_mrn"`

	// integration details
	Mrn   types.String `tfsdk:"mrn"`
	Name  types.String `tfsdk:"name"`
	Token types.String `tfsdk:"token"`

	// The GCP scope to scan (a folder ID or an organization ID).
	Scope types.String `tfsdk:"scope"`
	// The GCP project ID where the serverless scanner is deployed.
	HostProjectID types.String `tfsdk:"host_project_id"`
	// The GCP region where the serverless scanner is deployed.
	Region types.String `tfsdk:"region"`
	// (Optional.) A customer-provided service account identity to run the
	// integration with, instead of the platform creating one.
	SuppliedSaIdentity types.String `tfsdk:"supplied_sa_identity"`

	// Allow this integration to land scanned assets in spaces across multiple
	// orgs. Immutable after creation.
	CrossOrg types.Bool `tfsdk:"cross_org"`
	// When true, the deployed scanner authenticates back to the platform via GCP
	// Workload Identity Federation. Immutable after creation.
	UseWif types.Bool `tfsdk:"use_wif"`
	// The numeric unique ID of the GCP service account the deployed scanner runs
	// as, used as the WIF binding subject. Immutable after creation.
	ServiceAccountID types.String `tfsdk:"service_account_id"`

	// (Computed.) MRN of the server-managed WIF auth binding created for this
	// integration. Empty when use_wif is false.
	WifAuthBindingMrn types.String `tfsdk:"wif_auth_binding_mrn"`
	// (Computed.) Base64-encoded WIF external account configuration for the
	// deployed scanner. Pass it to the customer's Terraform deployment. Empty
	// when use_wif is false.
	WifConfig types.String `tfsdk:"wif_config"`

	// (Optional.)
	ScanConfiguration *GcpServerlessScanConfigurationInput `tfsdk:"scan_configuration"`
}

type GcpServerlessScanConfigurationInput struct {
	// (Optional.) Include filter: only projects whose tags match at least one of
	// these key-value pairs are scanned. A value of "*" matches any value for the key.
	TagsFilter types.Map `tfsdk:"tags_filter"`
	// (Optional.) Exclude filter: projects whose tags match at least one of these
	// key-value pairs are skipped. A value of "*" matches any value for the key.
	ExcludedTagsFilter types.Map `tfsdk:"excluded_tags_filter"`
	// (Optional.) How often (in hours) the deployed scanner runs a scan.
	ScanScheduleHours types.Int32 `tfsdk:"scan_schedule_hours"`
	// (Optional.) When true, the GCP project tags are propagated to all assets
	// discovered under the project.
	PropagateProjectTags types.Bool `tfsdk:"propagate_project_tags"`
}

// tagsFilterToKeyValueList converts a Terraform string map into the GraphQL
// list of key-value pairs expected by the API. It returns nil when the map is
// empty so the field is omitted from the request.
func tagsFilterToKeyValueList(m types.Map) *[]mondoov1.KeyValueInput {
	if m.IsNull() || m.IsUnknown() || len(m.Elements()) == 0 {
		return nil
	}

	kvs := make([]mondoov1.KeyValueInput, 0, len(m.Elements()))
	for k, v := range m.Elements() {
		value := strings.Trim(v.String(), "\"")
		kvs = append(kvs, mondoov1.KeyValueInput{
			Key:   mondoov1.String(k),
			Value: mondoov1.NewStringPtr(mondoov1.String(value)),
		})
	}
	return &kvs
}

func (m integrationGcpServerlessResourceModel) GetConfigurationOptions() *mondoov1.GcpServerlessConfigurationOptionsInput {
	opts := &mondoov1.GcpServerlessConfigurationOptionsInput{
		HostProjectId: mondoov1.String(m.HostProjectID.ValueString()),
		Region:        mondoov1.String(m.Region.ValueString()),
	}

	// Omitted scope means the scanner falls back to its default scope.
	if scope := m.Scope.ValueString(); scope != "" {
		opts.Scope = mondoov1.NewStringPtr(mondoov1.String(scope))
	}

	// Omitted means the platform creates the integration's identity itself.
	if sa := m.SuppliedSaIdentity.ValueString(); sa != "" {
		opts.SuppliedSaIdentity = mondoov1.NewStringPtr(mondoov1.String(sa))
	}

	if !m.CrossOrg.IsNull() && !m.CrossOrg.IsUnknown() {
		opts.CrossOrg = mondoov1.NewBooleanPtr(mondoov1.Boolean(m.CrossOrg.ValueBool()))
	}
	if !m.UseWif.IsNull() && !m.UseWif.IsUnknown() {
		opts.UseWif = mondoov1.NewBooleanPtr(mondoov1.Boolean(m.UseWif.ValueBool()))
	}
	if !m.ServiceAccountID.IsNull() && !m.ServiceAccountID.IsUnknown() {
		opts.ServiceAccountId = mondoov1.NewStringPtr(mondoov1.String(m.ServiceAccountID.ValueString()))
	}

	if m.ScanConfiguration != nil {
		opts.ScanConfiguration = &mondoov1.GcpServerlessScanConfigurationInput{
			TagsFilter:         tagsFilterToKeyValueList(m.ScanConfiguration.TagsFilter),
			ExcludedTagsFilter: tagsFilterToKeyValueList(m.ScanConfiguration.ExcludedTagsFilter),
		}
		if !m.ScanConfiguration.ScanScheduleHours.IsNull() && !m.ScanConfiguration.ScanScheduleHours.IsUnknown() {
			opts.ScanConfiguration.ScanScheduleHours = mondoov1.NewIntPtr(mondoov1.Int(m.ScanConfiguration.ScanScheduleHours.ValueInt32()))
		}
		if !m.ScanConfiguration.PropagateProjectTags.IsNull() && !m.ScanConfiguration.PropagateProjectTags.IsUnknown() {
			opts.ScanConfiguration.PropagateProjectTags = mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.PropagateProjectTags.ValueBool()))
		}
	}

	return opts
}

func (r *integrationGcpServerlessResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_gcp_serverless"
}

func (r *integrationGcpServerlessResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan GCP organizations and folders for misconfigurations and vulnerabilities using a serverless scanner deployed into your own host project.`,
		Attributes: map[string]schema.Attribute{
			"scope_mrn": schema.StringAttribute{
				MarkdownDescription: "The MRN of the scope (space, organization, or platform) the integration is created under (e.g. `//captain.api.mondoo.app/organizations/<org-id>`). When omitted, the provider's configured space is used. Required for organization-scoped / cross-org integrations. Immutable: changing it forces a new integration.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mrn": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Integration identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Integration token. Pass this to the serverless scanner deployment to register it with this integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the integration.",
				Required:            true,
			},
			"scope": schema.StringAttribute{
				MarkdownDescription: "The GCP scope to scan. Accepts either a folder ID or an organization ID. When omitted, the scanner falls back to its default scope.",
				Optional:            true,
			},
			"host_project_id": schema.StringAttribute{
				MarkdownDescription: "The GCP project ID where the serverless scanner is deployed.",
				Required:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The GCP region where the serverless scanner is deployed.",
				Required:            true,
			},
			"supplied_sa_identity": schema.StringAttribute{
				MarkdownDescription: "A customer-provided service account identity to run this integration with, instead of the platform automatically creating one (bring-your-own-identity). Stored and returned verbatim.",
				Optional:            true,
			},
			"cross_org": schema.BoolAttribute{
				MarkdownDescription: "Allow this integration to land scanned assets in spaces across multiple orgs. Only valid on organization-scoped integrations and only on private-instance deployments (rejected on prod and prod-eu). Requires `use_wif`. Immutable: changing it forces a new integration.",
				Optional:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"use_wif": schema.BoolAttribute{
				MarkdownDescription: "When true, the deployed scanner authenticates back to the platform via GCP Workload Identity Federation: a WIF auth binding is minted at create time. Immutable: changing it forces a new integration.",
				Optional:            true,
				PlanModifiers: []planmodifier.Bool{
					boolplanmodifier.RequiresReplace(),
				},
			},
			"service_account_id": schema.StringAttribute{
				MarkdownDescription: "The numeric unique ID of the GCP service account the deployed scanner runs as, used as the WIF binding subject. Required when `use_wif` is true. Immutable: changing it forces a new integration.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"wif_auth_binding_mrn": schema.StringAttribute{
				MarkdownDescription: "MRN of the server-managed WIF auth binding created for this integration. Empty when `use_wif` is false.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"wif_config": schema.StringAttribute{
				MarkdownDescription: "Base64-encoded WIF external account configuration for the deployed scanner. Pass it to the customer's Terraform deployment. Empty when `use_wif` is false.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"scan_configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "Scan options that control what the deployed scanner scans.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"tags_filter": schema.MapAttribute{
						MarkdownDescription: "Include filter: when not empty, only projects whose tags match at least one of these key-value pairs are scanned. A value of `*` matches any value for that tag key.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"excluded_tags_filter": schema.MapAttribute{
						MarkdownDescription: "Exclude filter: projects whose tags match at least one of these key-value pairs are skipped, even if they match the include filter. A value of `*` matches any value for that tag key.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"scan_schedule_hours": schema.Int32Attribute{
						MarkdownDescription: "How often (in hours) the deployed scanner runs a scan. Must be between 1 and 23.",
						Optional:            true,
						Validators: []validator.Int32{
							int32validator.Between(1, 23),
						},
					},
					"propagate_project_tags": schema.BoolAttribute{
						MarkdownDescription: "When true, the GCP project tags are propagated to all assets discovered under the project.",
						Optional:            true,
					},
				},
			},
		},
	}
}

func (r *integrationGcpServerlessResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ValidateConfig enforces the WIF / cross-org preconditions at plan time so
// misconfigurations surface during `terraform validate` instead of failing at
// the API on apply.
func (r *integrationGcpServerlessResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data integrationGcpServerlessResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateGcpServerlessConfig(&data)...)
}

// validateGcpServerlessConfig holds the WIF / cross-org precondition checks.
// Unknown values (references to other resources) are skipped so validation does
// not fire before the value is resolved.
func validateGcpServerlessConfig(data *integrationGcpServerlessResourceModel) (diagnostics diag.Diagnostics) {
	// use_wif requires a service_account_id (the WIF binding subject).
	if data.UseWif.ValueBool() && !data.ServiceAccountID.IsUnknown() && data.ServiceAccountID.ValueString() == "" {
		diagnostics.AddAttributeError(
			path.Root("service_account_id"),
			"Missing service_account_id",
			"service_account_id is required when use_wif is set to true.",
		)
	}

	// cross_org mints a platform-scoped WIF binding, so it requires use_wif.
	if data.CrossOrg.ValueBool() && !data.UseWif.IsUnknown() && !data.UseWif.ValueBool() {
		diagnostics.AddAttributeError(
			path.Root("use_wif"),
			"cross_org requires use_wif",
			"use_wif must be set to true when cross_org is enabled.",
		)
	}

	// cross_org is only valid on an organization-scoped integration, so
	// scope_mrn must be set to an organization MRN. A null/empty scope_mrn
	// (which would fall back to the provider space) and a space-scoped MRN are
	// both rejected, with distinct messages.
	if data.CrossOrg.ValueBool() && !data.ScopeMrn.IsUnknown() {
		switch scope := data.ScopeMrn.ValueString(); {
		case scope == "":
			diagnostics.AddAttributeError(
				path.Root("scope_mrn"),
				"cross_org requires an explicit scope_mrn",
				"scope_mrn is required when cross_org is enabled; set it to an organization MRN "+
					"(e.g. //captain.api.mondoo.app/organizations/<org-id>). Omitting it uses the provider "+
					"space, which is not organization-scoped.",
			)
		case !strings.HasPrefix(scope, orgPrefix):
			diagnostics.AddAttributeError(
				path.Root("scope_mrn"),
				"cross_org requires an organization scope",
				"cross_org can only be set on an organization-scoped integration; set scope_mrn to an organization MRN "+
					"(e.g. //captain.api.mondoo.app/organizations/<org-id>).",
			)
		}
	}
	return diagnostics
}

func (r *integrationGcpServerlessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationGcpServerlessResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	configInput := mondoov1.ClientIntegrationConfigurationInput{
		GcpServerlessConfigurationOptions: data.GetConfigurationOptions(),
	}

	// Resolve the scope. An explicit scope_mrn (space, organization, or
	// platform) is used directly — org scope is what enables cross_org
	// integrations. When omitted, fall back to the provider's configured space.
	var integration *CreateClientIntegrationPayload
	var err error
	if scopeMrn := data.ScopeMrn.ValueString(); scopeMrn != "" {
		ctx = tflog.SetField(ctx, "scope_mrn", scopeMrn)
		tflog.Debug(ctx, "Creating integration")
		integration, err = r.client.CreateScopedIntegration(ctx,
			scopeMrn,
			data.Name.ValueString(),
			mondoov1.ClientIntegrationTypeGcpServerless,
			configInput)
		if err != nil {
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to create GCP serverless integration. Got error: %s", err))
			return
		}
		data.ScopeMrn = types.StringValue(scopeMrn)
	} else {
		space, spaceErr := r.client.ComputeSpace(types.StringNull())
		if spaceErr != nil {
			resp.Diagnostics.AddError("Invalid Configuration", spaceErr.Error())
			return
		}
		ctx = tflog.SetField(ctx, "space_mrn", space.MRN())
		tflog.Debug(ctx, "Creating integration")
		integration, err = r.client.CreateIntegration(ctx,
			space.MRN(),
			data.Name.ValueString(),
			mondoov1.ClientIntegrationTypeGcpServerless,
			configInput)
		if err != nil {
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to create GCP serverless integration. Got error: %s", err))
			return
		}
		data.ScopeMrn = types.StringValue(space.MRN())
	}

	// trigger integration to gather results quickly after the first setup
	// NOTE: we ignore the error since the integration state does not depend on it
	_, err = r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunScan)
	if err != nil {
		resp.Diagnostics.
			AddWarning("Client Error",
				fmt.Sprintf("Unable to trigger integration. Got error: %s", err),
			)
	}

	// Save into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))
	data.Token = types.StringValue(string(integration.Token))

	// Fetch the full integration to populate the server-computed WIF fields
	// (wif_config / wif_auth_binding_mrn), which are minted at create time.
	r.refreshServerState(ctx, string(integration.Mrn), &data, &resp.Diagnostics)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// refreshServerState fetches the integration and reconciles the
// server-computed fields onto the model: the resolved scope and the
// server-managed WIF outputs. User-authoritative fields (e.g. name) are left
// as-is. Computed attributes must be set to known values, so on error the WIF
// outputs fall back to empty strings.
func (r *integrationGcpServerlessResource) refreshServerState(ctx context.Context, mrn string, data *integrationGcpServerlessResourceModel, diags *diag.Diagnostics) {
	fetched, err := r.client.GetClientIntegration(ctx, mrn)
	if err != nil {
		diags.AddWarning("Client Warning",
			fmt.Sprintf("Unable to fetch integration to populate computed fields. Got error: %s", err))
		data.WifConfig = types.StringValue("")
		data.WifAuthBindingMrn = types.StringValue("")
		return
	}
	// Reconcile the resolved scope so it is populated after import. scope_mrn is
	// immutable (RequiresReplace), so refreshing it here can't clobber a pending
	// change. name is intentionally NOT refreshed: it is user-authoritative and
	// mutable, so overwriting it would suppress a rename diff.
	if scope := fetched.ScopeMRN(); scope != "" {
		data.ScopeMrn = types.StringValue(scope)
	}

	// GcpServerlessConfigurationOptions is a value type in the union (not a
	// pointer), so there is nothing to nil-check. If use_wif is set but the
	// server returned an empty WIF config, surface it as a warning rather than
	// silently masking the real value with an empty string.
	opts := fetched.ConfigurationOptions.GcpServerlessConfigurationOptions
	if data.UseWif.ValueBool() && opts.WifConfig == "" {
		diags.AddWarning("Client Warning",
			"use_wif is enabled but the server returned an empty WIF config; the deployed scanner may not be able to authenticate.")
	}
	data.WifConfig = types.StringValue(opts.WifConfig)
	data.WifAuthBindingMrn = types.StringValue(opts.WifAuthBindingMrn)
}

func (r *integrationGcpServerlessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationGcpServerlessResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Refresh the server-computed WIF fields.
	r.refreshServerState(ctx, data.Mrn.ValueString(), &data, &resp.Diagnostics)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGcpServerlessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationGcpServerlessResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		GcpServerlessConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeGcpServerless,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update GCP serverless integration. Got error: %s", err),
			)
		return
	}

	// Refresh the server-computed WIF fields.
	r.refreshServerState(ctx, data.Mrn.ValueString(), &data, &resp.Diagnostics)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGcpServerlessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationGcpServerlessResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf(
					"Unable to delete GCP serverless integration '%s'. Got error: %s",
					data.Mrn.ValueString(), err.Error(),
				),
			)
		return
	}
}

func (r *integrationGcpServerlessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	model := integrationGcpServerlessResourceModel{
		Mrn:      types.StringValue(integration.Mrn),
		Name:     types.StringValue(integration.Name),
		ScopeMrn: types.StringValue(integration.ScopeMRN()),
	}

	resp.State.Set(ctx, &model)
}
