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
var _ resource.Resource = (*integrationZendeskResource)(nil)
var _ resource.ResourceWithImportState = (*integrationZendeskResource)(nil)

func NewIntegrationZendeskResource() resource.Resource {
	return &integrationZendeskResource{}
}

type integrationZendeskResource struct {
	client *ExtendedGqlClient
}

type integrationZendeskResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn       types.String `tfsdk:"mrn"`
	Name      types.String `tfsdk:"name"`
	Subdomain types.String `tfsdk:"subdomain"`
	Email     types.String `tfsdk:"email"`

	// (Optional.)
	AutoClose    types.Bool                            `tfsdk:"auto_close"`
	AutoCreate   types.Bool                            `tfsdk:"auto_create"`
	CustomFields *[]integrationZendeskCustomFieldModel `tfsdk:"custom_fields"`

	// credentials
	Credential *integrationZendeskCredentialModel `tfsdk:"credentials"`
}

type integrationZendeskCustomFieldModel struct {
	ID    types.Int64  `tfsdk:"id"`
	Value types.String `tfsdk:"value"`
}

type integrationZendeskCredentialModel struct {
	Token types.String `tfsdk:"token"`
}

func (m integrationZendeskResourceModel) GetConfigurationOptions() *mondoov1.ZendeskConfigurationOptionsInput {
	opts := &mondoov1.ZendeskConfigurationOptionsInput{
		Subdomain:         mondoov1.String(m.Subdomain.ValueString()),
		Email:             mondoov1.String(m.Email.ValueString()),
		AutoCloseTickets:  mondoov1.Boolean(m.AutoClose.ValueBool()),
		AutoCreateTickets: mondoov1.Boolean(m.AutoCreate.ValueBool()),
		CustomFields:      convertCustomFields(m.CustomFields),
		ApiToken:          mondoov1.String(m.Credential.Token.ValueString()),
	}

	return opts
}

func (r *integrationZendeskResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_zendesk"
}

func (r *integrationZendeskResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Zendesk integration to keep track of security tasks and add Zendesk tickets directly from within the Mondoo Console.`,
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
			"subdomain": schema.StringAttribute{
				MarkdownDescription: "Zendesk subdomain.",
				Required:            true,
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "Zendesk email.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
						"must be a valid email",
					),
				},
			},
			"auto_close": schema.BoolAttribute{
				MarkdownDescription: "Automatically close tickets.",
				Optional:            true,
			},
			"auto_create": schema.BoolAttribute{
				MarkdownDescription: "Automatically create tickets.",
				Optional:            true,
			},
			"custom_fields": schema.ListNestedAttribute{
				MarkdownDescription: "Custom fields to add to the Zendesk ticket.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"id": schema.Int64Attribute{
							MarkdownDescription: "Custom field ID.",
							Required:            true,
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "Custom field value.",
							Required:            true,
						},
					},
				},
			},
			"credentials": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"token": schema.StringAttribute{
						MarkdownDescription: "Token for Zendesk integration.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
		},
	}
}

func (r *integrationZendeskResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func convertCustomFields(recipients *[]integrationZendeskCustomFieldModel) *[]mondoov1.ZendeskCustomFieldInput {
	if recipients == nil {
		return nil
	}
	var result []mondoov1.ZendeskCustomFieldInput
	for _, r := range *recipients {
		result = append(result, mondoov1.ZendeskCustomFieldInput{
			Id:    mondoov1.Int(r.ID.ValueInt64()),
			Value: mondoov1.String(r.Value.ValueString()),
		})
	}
	return &result
}

func (r *integrationZendeskResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationZendeskResourceModel

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
		mondoov1.ClientIntegrationTypeTicketSystemZendesk,
		mondoov1.ClientIntegrationConfigurationInput{
			ZendeskConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create Zendesk integration. Got error: %s", err),
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

func (r *integrationZendeskResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationZendeskResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationZendeskResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationZendeskResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		ZendeskConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeTicketSystemZendesk,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update Zendesk integration. Got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationZendeskResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationZendeskResourceModel

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
				fmt.Sprintf("Unable to delete Zendesk integration. Got error: %s", err),
			)
		return
	}
}

func (r *integrationZendeskResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	var customFields []integrationZendeskCustomFieldModel
	for _, field := range integration.ConfigurationOptions.ZendeskConfigurationOptions.CustomFields {
		customFields = append(customFields, integrationZendeskCustomFieldModel{
			ID:    types.Int64Value(field.ID),
			Value: types.StringValue(field.Value),
		})
	}

	model := integrationZendeskResourceModel{
		Mrn:          types.StringValue(integration.Mrn),
		Name:         types.StringValue(integration.Name),
		SpaceID:      types.StringValue(integration.SpaceID()),
		Subdomain:    types.StringValue(integration.ConfigurationOptions.ZendeskConfigurationOptions.Subdomain),
		Email:        types.StringValue(integration.ConfigurationOptions.ZendeskConfigurationOptions.Email),
		AutoClose:    types.BoolValue(integration.ConfigurationOptions.ZendeskConfigurationOptions.AutoCloseTickets),
		AutoCreate:   types.BoolValue(integration.ConfigurationOptions.ZendeskConfigurationOptions.AutoCreateTickets),
		CustomFields: &customFields,
		Credential: &integrationZendeskCredentialModel{
			Token: types.StringPointerValue(nil),
		},
	}

	resp.State.Set(ctx, &model)
}
