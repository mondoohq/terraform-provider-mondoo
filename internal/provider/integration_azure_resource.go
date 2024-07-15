package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var _ resource.Resource = (*integrationAzureResource)(nil)

func NewIntegrationAzureResource() resource.Resource {
	return &integrationAzureResource{}
}

type integrationAzureResource struct {
	client *ExtendedGqlClient
}

type integrationAzureResourceModel struct {
	// scope
	SpaceId types.String `tfsdk:"space_id"`

	// integration details
	Mrn                   types.String `tfsdk:"mrn"`
	Name                  types.String `tfsdk:"name"`
	ClientId              types.String `tfsdk:"client_id"`
	TenantId              types.String `tfsdk:"tenant_id"`
	SubscriptionAllowList types.List   `tfsdk:"subscription_allow_list"`
	SubscriptionDenyList  types.List   `tfsdk:"subscription_deny_list"`
	ScanVms               types.Bool   `tfsdk:"scan_vms"`

	// credentials
	Credential integrationAzureCredentialModel `tfsdk:"credentials"`
}

type integrationAzureCredentialModel struct {
	PEMFile types.String `tfsdk:"pem_file"`
}

func (r *integrationAzureResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_azure"
}

func (r *integrationAzureResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan Microsoft Azure subscriptions and resources for misconfigurations and vulnerabilities. See [Mondoo documentation](https://mondoo.com/docs/platform/infra/cloud/azure/azure-integration-scan-subscription/) for more details.`,
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo Space Identifier.",
				Required:            true,
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
				MarkdownDescription: "Azure Client ID.",
				Required:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Azure Tenant ID.",
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
			"credentials": schema.SingleNestedAttribute{
				Required: true,
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

	client, ok := req.ProviderData.(*mondoov1.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = &ExtendedGqlClient{client}
}

func (r *integrationAzureResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

	var data integrationAzureResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource.
	spaceMrn := ""
	if data.SpaceId.ValueString() != "" {
		spaceMrn = spacePrefix + data.SpaceId.ValueString()
	}

	var listAllow []mondoov1.String
	allowlist, _ := data.SubscriptionAllowList.ToListValue(ctx)
	allowlist.ElementsAs(ctx, &listAllow, true)

	var listDeny []mondoov1.String
	denylist, _ := data.SubscriptionDenyList.ToListValue(ctx)
	denylist.ElementsAs(ctx, &listDeny, true)

	// Check if both whitelist and blacklist are provided
	if len(listDeny) > 0 && len(listAllow) > 0 {
		resp.Diagnostics.AddError("ConflictingAttributesError", "Both subscription_allow_list and subscription_deny_list cannot be provided simultaneously.")
		return
	}

	integration, err := r.client.CreateIntegration(ctx,
		spaceMrn,
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAzure,
		mondoov1.ClientIntegrationConfigurationInput{
			AzureConfigurationOptions: &mondoov1.AzureConfigurationOptionsInput{
				TenantID:               mondoov1.String(data.TenantId.ValueString()),
				ClientID:               mondoov1.String(data.ClientId.ValueString()),
				SubscriptionsWhitelist: &listAllow,
				SubscriptionsBlacklist: &listDeny,
				ScanVms:                mondoov1.NewBooleanPtr(mondoov1.Boolean(data.ScanVms.ValueBool())),
				Certificate:            mondoov1.NewStringPtr(mondoov1.String(data.Credential.PEMFile.ValueString())),
			},
		})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create Azure integration, got error: %s", err))
		return
	}

	// trigger integration to gather results quickly after the first setup
	// NOTE: we ignore the error since the integration state does not depend on it
	_, err = r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunScan)
	if err != nil {
		resp.Diagnostics.AddWarning("Client Error", fmt.Sprintf("Unable to trigger integration, got error: %s", err))
		return
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))
	data.SpaceId = types.StringValue(data.SpaceId.ValueString())

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

	// Read API call logic

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
		resp.Diagnostics.AddError("ConflictingAttributesError", "Both subscription_allow_list and subscription_deny_list cannot be provided simultaneously.")
		return
	}

	opts := mondoov1.ClientIntegrationConfigurationInput{
		AzureConfigurationOptions: &mondoov1.AzureConfigurationOptionsInput{
			TenantID:               mondoov1.String(data.TenantId.ValueString()),
			ClientID:               mondoov1.String(data.ClientId.ValueString()),
			SubscriptionsWhitelist: &listAllow,
			SubscriptionsBlacklist: &listDeny,
			ScanVms:                mondoov1.NewBooleanPtr(mondoov1.Boolean(data.ScanVms.ValueBool())),
			Certificate:            mondoov1.NewStringPtr(mondoov1.String(data.Credential.PEMFile.ValueString())),
		},
	}

	_, err := r.client.UpdateIntegration(ctx, data.Mrn.ValueString(), data.Name.ValueString(), mondoov1.ClientIntegrationTypeAzure, opts)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update Azure integration, got error: %s", err))
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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete Azure integration, got error: %s", err))
		return
	}
}

func (r *integrationAzureResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	mrn := req.ID
	integration, err := r.client.GetClientIntegration(ctx, mrn)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to import Azure integration, got error: %s", err))
		return
	}

	allowList := r.ConvertListValue(ctx, integration.ConfigurationOptions.AzureConfigurationOptions.SubscriptionsWhitelist)
	denyList := r.ConvertListValue(ctx, integration.ConfigurationOptions.AzureConfigurationOptions.SubscriptionsBlacklist)

	model := integrationAzureResourceModel{
		SpaceId:               types.StringValue(strings.Split(integration.Mrn, "/")[len(strings.Split(integration.Mrn, "/"))-3]),
		Mrn:                   types.StringValue(integration.Mrn),
		Name:                  types.StringValue(integration.Name),
		ClientId:              types.StringValue(integration.ConfigurationOptions.AzureConfigurationOptions.ClientId),
		TenantId:              types.StringValue(integration.ConfigurationOptions.AzureConfigurationOptions.TenantId),
		SubscriptionAllowList: allowList,
		SubscriptionDenyList:  denyList,
		Credential: integrationAzureCredentialModel{
			PEMFile: types.StringPointerValue(nil),
		},
		ScanVms: types.BoolValue(integration.ConfigurationOptions.AzureConfigurationOptions.ScanVms),
	}

	resp.State.Set(ctx, &model)
}

func (r *integrationAzureResource) ConvertListValue(ctx context.Context, list []string) types.List {
	var valueList []attr.Value
	for _, str := range list {
		valueList = append(valueList, types.StringValue(str))
	}
	// Ensure the list is of type types.StringType
	return types.ListValueMust(types.StringType, valueList)
}
