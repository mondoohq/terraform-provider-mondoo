package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
var _ resource.Resource = (*integrationWebhookResource)(nil)
var _ resource.ResourceWithImportState = (*integrationWebhookResource)(nil)

func NewIntegrationWebhookResource() resource.Resource {
	return &integrationWebhookResource{}
}

type integrationWebhookResource struct {
	client *ExtendedGqlClient
}

type integrationWebhookResourceModel struct {
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn  types.String `tfsdk:"mrn"`
	Name types.String `tfsdk:"name"`
	Url  types.String `tfsdk:"url"`

	// Optional settings
	AutoCreate types.Bool `tfsdk:"auto_create"`
	AutoClose  types.Bool `tfsdk:"auto_close"`
}

func (r *integrationWebhookResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_webhook"
}

func (m integrationWebhookResourceModel) GetConfigurationOptions() *mondoov1.WebhookConfigurationOptionsInput {
	opts := &mondoov1.WebhookConfigurationOptionsInput{
		Url:               mondoov1.String(m.Url.ValueString()),
		AutoCreateTickets: mondoov1.Boolean(m.AutoCreate.ValueBool()),
		AutoCloseTickets:  mondoov1.Boolean(m.AutoClose.ValueBool()),
	}

	return opts
}

func (r *integrationWebhookResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Integrate a webhook-based ticket system with Mondoo to automatically create and close tickets based on Mondoo findings.`,
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
			"url": schema.StringAttribute{
				MarkdownDescription: "Webhook URL that will be called for ticket operations.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^https?:\/\/[a-zA-Z0-9\-._~:\/?#[\]@!$&'()*+,;=%]+$`),
						"must be a valid URL",
					),
				},
			},
			"auto_create": schema.BoolAttribute{
				MarkdownDescription: "Automatically create tickets for new Mondoo findings.",
				Optional:            true,
			},
			"auto_close": schema.BoolAttribute{
				MarkdownDescription: "Automatically close tickets when Mondoo findings are resolved.",
				Optional:            true,
			},
		},
	}
}

func (r *integrationWebhookResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationWebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationWebhookResourceModel

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

	// Do GraphQL request to API to create the resource.
	tflog.Debug(ctx, "Creating integration")
	integration, err := r.client.CreateIntegration(ctx,
		space.MRN(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeTicketSystemWebhook,
		mondoov1.ClientIntegrationConfigurationInput{
			WebhookConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create webhook integration. Got error: %s", err),
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

func (r *integrationWebhookResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationWebhookResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationWebhookResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationWebhookResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		WebhookConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeTicketSystemWebhook,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update webhook integration. Got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationWebhookResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationWebhookResourceModel

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
				fmt.Sprintf("Unable to delete webhook integration. Got error: %s", err),
			)
		return
	}
}

func (r *integrationWebhookResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	model := integrationWebhookResourceModel{
		Mrn:        types.StringValue(integration.Mrn),
		Name:       types.StringValue(integration.Name),
		SpaceID:    types.StringValue(integration.SpaceID()),
		Url:        types.StringValue(integration.ConfigurationOptions.WebhookConfigurationOptions.Url),
		AutoCreate: types.BoolValue(integration.ConfigurationOptions.WebhookConfigurationOptions.AutoCreateTickets),
		AutoClose:  types.BoolValue(integration.ConfigurationOptions.WebhookConfigurationOptions.AutoCloseTickets),
	}

	resp.State.Set(ctx, &model)
}
