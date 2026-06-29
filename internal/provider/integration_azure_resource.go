// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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
var _ resource.Resource = (*integrationAzureResource)(nil)
var _ resource.ResourceWithImportState = (*integrationAzureResource)(nil)
var _ resource.ResourceWithConfigValidators = (*integrationAzureResource)(nil)

func NewIntegrationAzureResource() resource.Resource {
	return &integrationAzureResource{}
}

type integrationAzureResource struct {
	client *ExtendedGqlClient
}

type integrationAzureResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn                   types.String `tfsdk:"mrn"`
	Name                  types.String `tfsdk:"name"`
	ClientId              types.String `tfsdk:"client_id"`
	TenantId              types.String `tfsdk:"tenant_id"`
	SubscriptionAllowList types.List   `tfsdk:"subscription_allow_list"`
	SubscriptionDenyList  types.List   `tfsdk:"subscription_deny_list"`
	ScanVms               types.Bool   `tfsdk:"scan_vms"`

	// WIF (Workload Identity Federation) — keyless mode
	UseWif       types.Bool   `tfsdk:"use_wif"`
	WifSubject   types.String `tfsdk:"wif_subject"`
	WifIssuerUrl types.String `tfsdk:"wif_issuer_url"`

	// credentials — certificate mode (nil when use_wif=true)
	Credential *integrationAzureCredentialModel `tfsdk:"credentials"`
}

type integrationAzureCredentialModel struct {
	PEMFile types.String `tfsdk:"pem_file"`
}

func (r *integrationAzureResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_azure"
}

// ConfigValidators enforces the mutual exclusion between use_wif and credentials.
// Exactly one of the two must be configured.
func (r *integrationAzureResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("use_wif"),
			path.MatchRoot("credentials"),
		),
	}
}

func (r *integrationAzureResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan Microsoft Azure subscriptions and resources for misconfigurations and vulnerabilities. To learn more, read the [Mondoo documentation](https://mondoo.com/docs/platform/infra/cloud/azure/azure-integration-scan-subscription/).`,
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
			"scan_vms": schema.BoolAttribute{
				MarkdownDescription: "Scan VMs.",
				Optional:            true,
			},
			"subscription_allow_list": schema.ListAttribute{
				MarkdownDescription: "List of Azure subscriptions to scan.",
				Optional:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					// Validate only this attribute or other_attr is configured.
					listvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot("subscription_deny_list"),
					}...),
				},
			},
			"subscription_deny_list": schema.ListAttribute{
				MarkdownDescription: "List of Azure subscriptions to exclude from scanning.",
				Optional:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					// Validate only this attribute or other_attr is configured.
					listvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot("subscription_allow_list"),
					}...),
				},
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
				MarkdownDescription: "Certificate credentials for Azure integration. Mutually exclusive with `use_wif`.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"pem_file": schema.StringAttribute{
						MarkdownDescription: "PEM file for Azure integration.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
		},
	}
}

func (r *integrationAzureResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationAzureResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

	var data integrationAzureResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "space_mrn", space.MRN())

	// Do GraphQL request to API to create the resource.
	var listAllow []mondoov1.String
	allowlist, _ := data.SubscriptionAllowList.ToListValue(ctx)
	allowlist.ElementsAs(ctx, &listAllow, true)

	var listDeny []mondoov1.String
	denylist, _ := data.SubscriptionDenyList.ToListValue(ctx)
	denylist.ElementsAs(ctx, &listDeny, true)

	// Check if both whitelist and blacklist are provided
	if len(listDeny) > 0 && len(listAllow) > 0 {
		resp.Diagnostics.
			AddError("ConflictingAttributesError",
				"You can't provide both a subscription_allow_list and a subscription_deny_list. Choose one or the other.",
			)
		return
	}

	// Build Azure configuration options — WIF mode or certificate mode.
	azureOpts := &mondoov1.AzureConfigurationOptionsInput{
		TenantId:               mondoov1.String(data.TenantId.ValueString()),
		ClientId:               mondoov1.String(data.ClientId.ValueString()),
		SubscriptionsWhitelist: &listAllow,
		SubscriptionsBlacklist: &listDeny,
		ScanVms:                mondoov1.NewBooleanPtr(mondoov1.Boolean(data.ScanVms.ValueBool())),
	}
	if data.UseWif.ValueBool() {
		azureOpts.UseWif = mondoov1.NewBooleanPtr(mondoov1.Boolean(true))
	} else if data.Credential != nil {
		azureOpts.Certificate = mondoov1.NewStringPtr(mondoov1.String(data.Credential.PEMFile.ValueString()))
	} else {
		resp.Diagnostics.AddError("Missing Azure credentials", "Set use_wif = true or provide a credentials block with pem_file.")
		return
	}

	tflog.Debug(ctx, "Creating integration")
	integration, err := r.client.CreateIntegration(ctx,
		space.MRN(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAzure,
		mondoov1.ClientIntegrationConfigurationInput{
			AzureConfigurationOptions: azureOpts,
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create Azure integration. Got error: %s", err),
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

	// Populate WIF computed fields from the integration response.
	// These are returned by the server after creation and used to wire up the
	// azuread_application_federated_identity_credential resource.
	fetchedIntegration, err := r.client.GetClientIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		// In WIF mode, empty wif_subject/wif_issuer_url would silently misconfigure
		// the downstream azuread_application_federated_identity_credential, so surface it.
		if data.UseWif.ValueBool() {
			resp.Diagnostics.AddWarning(
				"Unable to read WIF fields",
				fmt.Sprintf("The integration was created, but its Workload Identity Federation fields "+
					"(wif_subject, wif_issuer_url) could not be read back: %s. The federated identity "+
					"credential may receive empty values; re-run 'terraform apply' to refresh.", err),
			)
		}
	} else {
		data.WifSubject = types.StringValue(fetchedIntegration.ConfigurationOptions.AzureConfigurationOptions.WifSubject)
		data.WifIssuerUrl = types.StringValue(fetchedIntegration.ConfigurationOptions.AzureConfigurationOptions.WifIssuerUrl)
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAzureResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationAzureResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Fetch current state from the API to pick up computed WIF fields.
	if data.Mrn.ValueString() != "" {
		integration, err := r.client.GetClientIntegration(ctx, data.Mrn.ValueString())
		if err != nil {
			// Only drop the resource from state when it genuinely no longer
			// exists. A transient error (network, auth, server) must not cause
			// Terraform to delete the resource from state.
			if isNotFoundError(err) {
				resp.State.RemoveResource(ctx)
				return
			}
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to read Azure integration: %s", err),
			)
			return
		}
		data.WifSubject = types.StringValue(integration.ConfigurationOptions.AzureConfigurationOptions.WifSubject)
		data.WifIssuerUrl = types.StringValue(integration.ConfigurationOptions.AzureConfigurationOptions.WifIssuerUrl)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAzureResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationAzureResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	var listAllow []mondoov1.String
	allowlist, _ := data.SubscriptionAllowList.ToListValue(ctx)
	allowlist.ElementsAs(ctx, &listAllow, true)

	var listDeny []mondoov1.String
	denylist, _ := data.SubscriptionDenyList.ToListValue(ctx)
	denylist.ElementsAs(ctx, &listDeny, true)

	// Check if both whitelist and blacklist are provided
	if len(listDeny) > 0 && len(listAllow) > 0 {
		resp.Diagnostics.
			AddError("ConflictingAttributesError",
				"You can't provide both a subscription_allow_list and a subscription_deny_list. Choose one or the other.",
			)
		return
	}

	// Build Azure configuration options — WIF mode or certificate mode.
	azureOpts := &mondoov1.AzureConfigurationOptionsInput{
		TenantId:               mondoov1.String(data.TenantId.ValueString()),
		ClientId:               mondoov1.String(data.ClientId.ValueString()),
		SubscriptionsWhitelist: &listAllow,
		SubscriptionsBlacklist: &listDeny,
		ScanVms:                mondoov1.NewBooleanPtr(mondoov1.Boolean(data.ScanVms.ValueBool())),
	}
	if data.UseWif.ValueBool() {
		azureOpts.UseWif = mondoov1.NewBooleanPtr(mondoov1.Boolean(true))
	} else if data.Credential != nil {
		azureOpts.Certificate = mondoov1.NewStringPtr(mondoov1.String(data.Credential.PEMFile.ValueString()))
	} else {
		resp.Diagnostics.AddError("Missing Azure credentials", "Set use_wif = true or provide a credentials block with pem_file.")
		return
	}

	opts := mondoov1.ClientIntegrationConfigurationInput{
		AzureConfigurationOptions: azureOpts,
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAzure,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update Azure integration. Got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAzureResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationAzureResourceModel

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
				fmt.Sprintf("Unable to delete Azure integration. Got error: %s", err),
			)
		return
	}
}

func (r *integrationAzureResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	allowList := ConvertListValue(integration.ConfigurationOptions.AzureConfigurationOptions.SubscriptionsWhitelist)
	denyList := ConvertListValue(integration.ConfigurationOptions.AzureConfigurationOptions.SubscriptionsBlacklist)

	azureOpts := integration.ConfigurationOptions.AzureConfigurationOptions
	model := integrationAzureResourceModel{
		SpaceID:               types.StringValue(integration.SpaceID()),
		Mrn:                   types.StringValue(integration.Mrn),
		Name:                  types.StringValue(integration.Name),
		ClientId:              types.StringValue(azureOpts.ClientId),
		TenantId:              types.StringValue(azureOpts.TenantId),
		SubscriptionAllowList: allowList,
		SubscriptionDenyList:  denyList,
		ScanVms:               types.BoolValue(azureOpts.ScanVms),
		WifSubject:            types.StringValue(azureOpts.WifSubject),
		WifIssuerUrl:          types.StringValue(azureOpts.WifIssuerUrl),
	}

	// Detect the auth mode from the API response so the imported state satisfies
	// the ExactlyOneOf(use_wif, credentials) validator and matches the user's config.
	if azureOpts.WifSubject != "" {
		model.UseWif = types.BoolValue(true)
		model.Credential = nil
	} else {
		// Certificate mode: the PEM is write-only and cannot be read back, so it
		// is left null — re-supply it in config after import. Setting the
		// credentials block keeps the import aligned with a certificate config.
		model.UseWif = types.BoolNull()
		model.Credential = &integrationAzureCredentialModel{PEMFile: types.StringNull()}
	}

	resp.State.Set(ctx, &model)
}
