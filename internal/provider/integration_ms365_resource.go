// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
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
var _ resource.Resource = (*integrationMs365Resource)(nil)
var _ resource.ResourceWithImportState = (*integrationMs365Resource)(nil)
var _ resource.ResourceWithConfigValidators = (*integrationMs365Resource)(nil)

func NewIntegrationMs365Resource() resource.Resource {
	return &integrationMs365Resource{}
}

type integrationMs365Resource struct {
	client *ExtendedGqlClient
}

type integrationMs365ResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn      types.String `tfsdk:"mrn"`
	Name     types.String `tfsdk:"name"`
	ClientId types.String `tfsdk:"client_id"`
	TenantId types.String `tfsdk:"tenant_id"`

	// WIF (Workload Identity Federation) — keyless mode
	UseWif       types.Bool   `tfsdk:"use_wif"`
	WifSubject   types.String `tfsdk:"wif_subject"`
	WifIssuerUrl types.String `tfsdk:"wif_issuer_url"`

	// credentials — certificate mode (nil when use_wif=true)
	Credential *integrationMs365CredentialModel `tfsdk:"credentials"`
}

type integrationMs365CredentialModel struct {
	PEMFile types.String `tfsdk:"pem_file"`
}

func (r *integrationMs365Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_ms365"
}

// ConfigValidators enforces the mutual exclusion between use_wif and credentials.
// Exactly one of the two must be configured.
func (r *integrationMs365Resource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("use_wif"),
			path.MatchRoot("credentials"),
		),
	}
}

func (r *integrationMs365Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously monitor your Microsoft 365 resources for misconfigurations and vulnerabilities. To learn more, read the [Mondoo documentation](https://mondoo.com/docs/platform/infra/saas/ms365/ms365-auto/).`,
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
				Computed:            true,
				MarkdownDescription: "Integration identifier",
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
			"client_id": schema.StringAttribute{
				MarkdownDescription: "Azure client ID.",
				Required:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Azure tenant ID.",
				Required:            true,
			},
			"use_wif": schema.BoolAttribute{
				MarkdownDescription: "Use Workload Identity Federation (keyless) instead of a certificate. Mutually exclusive with `credentials`.",
				Optional:            true,
			},
			"wif_subject": schema.StringAttribute{
				MarkdownDescription: "The WIF subject (populated by Mondoo after creation). Use as the `subject` of the `azuread_application_federated_identity_credential` resource.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"wif_issuer_url": schema.StringAttribute{
				MarkdownDescription: "The WIF issuer URL (populated by Mondoo after creation). Use as the `issuer` of the `azuread_application_federated_identity_credential` resource.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"credentials": schema.SingleNestedAttribute{
				MarkdownDescription: "Certificate credentials for MS365 integration. Mutually exclusive with `use_wif`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"pem_file": schema.StringAttribute{
						MarkdownDescription: "PEM file for MS365 integration.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
		},
	}
}

func (r *integrationMs365Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ms365ConfigurationOptions builds the MS365 configuration input for either WIF
// or certificate mode. In WIF mode the certificate is deliberately left unset:
// the API rejects a request that carries both.
func ms365ConfigurationOptions(data integrationMs365ResourceModel, diags *diag.Diagnostics) (*mondoov1.Ms365ConfigurationOptionsInput, bool) {
	opts := &mondoov1.Ms365ConfigurationOptionsInput{
		TenantId: mondoov1.String(data.TenantId.ValueString()),
		ClientId: mondoov1.String(data.ClientId.ValueString()),
	}
	switch {
	case data.UseWif.ValueBool():
		opts.UseWif = mondoov1.NewBooleanPtr(mondoov1.Boolean(true))
	case data.Credential != nil:
		opts.Certificate = mondoov1.NewStringPtr(mondoov1.String(data.Credential.PEMFile.ValueString()))
	default:
		// ExactlyOneOf treats an explicit `use_wif = false` as configured, so
		// this branch is reachable without a credentials block. Guard it
		// instead of dereferencing a nil credentials model.
		diags.AddError("Missing MS365 credentials", "Set use_wif = true or provide a credentials block with pem_file.")
		return nil, false
	}
	return opts, true
}

// refreshWifState reconciles the server-computed WIF outputs onto the model.
// Computed attributes must be set to known values, so on a fetch error they
// fall back to empty strings; the error is returned so the caller can classify
// it (Read has to tell a genuine not-found from a transient failure).
func (r *integrationMs365Resource) refreshWifState(ctx context.Context, mrn string, data *integrationMs365ResourceModel, diags *diag.Diagnostics) error {
	fetched, err := r.client.GetClientIntegration(ctx, mrn)
	if err != nil {
		data.WifSubject = types.StringValue("")
		data.WifIssuerUrl = types.StringValue("")
		return err
	}

	// Ms365ConfigurationOptions is a value type in the union (not a pointer),
	// so there is nothing to nil-check.
	opts := fetched.ConfigurationOptions.Ms365ConfigurationOptions
	if data.UseWif.ValueBool() && opts.WifSubject == "" {
		diags.AddWarning("Client Warning",
			"use_wif is enabled but the server returned an empty WIF subject; the federated identity "+
				"credential built from wif_subject/wif_issuer_url will not authenticate.")
	}
	data.WifSubject = types.StringValue(opts.WifSubject)
	data.WifIssuerUrl = types.StringValue(opts.WifIssuerUrl)
	return nil
}

func (r *integrationMs365Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationMs365ResourceModel

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

	ms365Opts, ok := ms365ConfigurationOptions(data, &resp.Diagnostics)
	if !ok {
		return
	}

	// Do GraphQL request to API to create the resource.
	tflog.Debug(ctx, "Creating integration")
	integration, err := r.client.CreateIntegration(ctx,
		space.MRN(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeMs365,
		mondoov1.ClientIntegrationConfigurationInput{
			Ms365ConfigurationOptions: ms365Opts,
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create MS365 integration. Got error: %s", err),
			)
		return
	}

	// trigger integration to gather results quickly after the first setup
	// NOTE: we ignore the error since the integration state does not depend on it
	_, err = r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunScan)
	if err != nil {
		resp.Diagnostics.
			AddWarning("Client Error",
				fmt.Sprintf("Unable to trigger integration. Got error: %s", err),
			)
		return
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))
	data.SpaceID = types.StringValue(space.ID())

	// Populate the WIF computed fields from the integration response. The server
	// mints them at create time and they are what wires up the downstream
	// azuread_application_federated_identity_credential resource.
	if err := r.refreshWifState(ctx, data.Mrn.ValueString(), &data, &resp.Diagnostics); err != nil && data.UseWif.ValueBool() {
		// In WIF mode, empty wif_subject/wif_issuer_url would silently
		// misconfigure the federated credential, so surface it.
		resp.Diagnostics.AddWarning(
			"Unable to read WIF fields",
			fmt.Sprintf("The integration was created, but its Workload Identity Federation fields "+
				"(wif_subject, wif_issuer_url) could not be read back: %s. The federated identity "+
				"credential may receive empty values; re-run 'terraform apply' to refresh.", err),
		)
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationMs365Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationMs365ResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Fetch current state from the API to pick up the computed WIF fields.
	if data.Mrn.ValueString() != "" {
		if err := r.refreshWifState(ctx, data.Mrn.ValueString(), &data, &resp.Diagnostics); err != nil {
			// Only drop the resource from state when it genuinely no longer
			// exists. A transient error (network, auth, server) must not cause
			// Terraform to delete the resource from state.
			if isNotFoundError(err) {
				resp.State.RemoveResource(ctx)
				return
			}
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to read MS365 integration: %s", err),
			)
			return
		}
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationMs365Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationMs365ResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	ms365Opts, ok := ms365ConfigurationOptions(data, &resp.Diagnostics)
	if !ok {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		Ms365ConfigurationOptions: ms365Opts,
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeMs365,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update MS365 integration. Got error: %s", err),
			)
		return
	}

	// NOTE: the WIF fields are deliberately not re-read here. Terraform carries
	// the prior state value into the plan for a computed attribute, so writing a
	// different one during apply trips "Provider produced inconsistent result
	// after apply". Toggling use_wif does change them server side; the next Read
	// (refresh) picks that up, which is also how mondoo_integration_azure behaves.

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationMs365Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationMs365ResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to delete MS365 integration. Got error: %s", err),
			)
		return
	}
}

func (r *integrationMs365Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	ms365Opts := integration.ConfigurationOptions.Ms365ConfigurationOptions
	model := integrationMs365ResourceModel{
		Mrn:          types.StringValue(integration.Mrn),
		Name:         types.StringValue(integration.Name),
		SpaceID:      types.StringValue(integration.SpaceID()),
		TenantId:     types.StringValue(ms365Opts.TenantId),
		ClientId:     types.StringValue(ms365Opts.ClientId),
		WifSubject:   types.StringValue(ms365Opts.WifSubject),
		WifIssuerUrl: types.StringValue(ms365Opts.WifIssuerUrl),
	}

	// Detect the auth mode from the API response so the imported state satisfies
	// the ExactlyOneOf(use_wif, credentials) validator and matches the user's config.
	if ms365Opts.UseWif {
		model.UseWif = types.BoolValue(true)
		model.Credential = nil
	} else {
		// Certificate mode: the PEM is write-only and cannot be read back, so it
		// is left null — re-supply it in config after import. Setting the
		// credentials block keeps the import aligned with a certificate config.
		model.UseWif = types.BoolNull()
		model.Credential = &integrationMs365CredentialModel{PEMFile: types.StringNull()}
	}

	resp.State.Set(ctx, &model)
}
