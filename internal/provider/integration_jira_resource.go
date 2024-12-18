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

var _ resource.Resource = (*integrationJiraResource)(nil)

func NewIntegrationJiraResource() resource.Resource {
	return &integrationJiraResource{}
}

type integrationJiraResource struct {
	client *ExtendedGqlClient
}

type integrationJiraResourceModel struct {
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn   types.String `tfsdk:"mrn"`
	Name  types.String `tfsdk:"name"`
	Host  types.String `tfsdk:"host"`
	Email types.String `tfsdk:"email"`

	// Optional settings
	DefaultProject types.String `tfsdk:"default_project"`
	AutoCreate     types.Bool   `tfsdk:"auto_create"`
	AutoClose      types.Bool   `tfsdk:"auto_close"`

	// credentials
	Credential *integrationJiraCredentialModel `tfsdk:"credentials"`
}

type integrationJiraCredentialModel struct {
	Token types.String `tfsdk:"token"`
}

func (r *integrationJiraResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_jira"
}

func (m integrationJiraResourceModel) GetConfigurationOptions() *mondoov1.JiraConfigurationOptionsInput {
	opts := &mondoov1.JiraConfigurationOptionsInput{
		Host:             mondoov1.String(m.Host.ValueString()),
		Email:            mondoov1.String(m.Email.ValueString()),
		APIToken:         mondoov1.String(m.Credential.Token.ValueString()),
		DefaultProject:   mondoov1.String(m.DefaultProject.ValueString()),
		AutoCreateCases:  mondoov1.NewBooleanPtr(mondoov1.Boolean(m.AutoCreate.ValueBool())),
		AutoCloseTickets: mondoov1.NewBooleanPtr(mondoov1.Boolean(m.AutoClose.ValueBool())),
	}

	return opts
}

func (r *integrationJiraResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Integrate the Jira ticket system with Mondoo to automatically create and close Jira issues based on Mondoo findings.`,
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
			"host": schema.StringAttribute{
				MarkdownDescription: "Jira host URL.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^https?:\/\/[a-zA-Z0-9\-._~:\/?#[\]@!$&'()*+,;=%]+$`),
						"must be a valid URL",
					),
				},
			},
			"email": schema.StringAttribute{
				MarkdownDescription: "Jira user email.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
						"must be a valid email",
					),
				},
			},
			"default_project": schema.StringAttribute{
				MarkdownDescription: "Default Jira project (represented by the project key, such as `SEC` or `SECURITY`).",
				Optional:            true,
			},
			"auto_create": schema.BoolAttribute{
				MarkdownDescription: "Automatically create Jira issues for Mondoo findings.",
				Optional:            true,
			},
			"auto_close": schema.BoolAttribute{
				MarkdownDescription: "Automatically close Jira issues for resolved Mondoo findings",
				Optional:            true,
			},
			"credentials": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"token": schema.StringAttribute{
						MarkdownDescription: "Jira API token.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
		},
	}
}

func (r *integrationJiraResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationJiraResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationJiraResourceModel

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
		mondoov1.ClientIntegrationTypeTicketSystemJira,
		mondoov1.ClientIntegrationConfigurationInput{
			JiraConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create Jira integration. Got error: %s", err),
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

func (r *integrationJiraResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationJiraResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationJiraResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationJiraResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		JiraConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeTicketSystemJira,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update Jira integration. Got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationJiraResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationJiraResourceModel

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
				fmt.Sprintf("Unable to delete Jira integration. Got error: %s", err),
			)
		return
	}
}

func (r *integrationJiraResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	model := integrationJiraResourceModel{
		Mrn:            types.StringValue(integration.Mrn),
		Name:           types.StringValue(integration.Name),
		SpaceID:        types.StringValue(integration.SpaceID()),
		Host:           types.StringValue(integration.ConfigurationOptions.JiraConfigurationOptions.Host),
		Email:          types.StringValue(integration.ConfigurationOptions.JiraConfigurationOptions.Email),
		DefaultProject: types.StringValue(integration.ConfigurationOptions.JiraConfigurationOptions.DefaultProject),
		AutoCreate:     types.BoolValue(integration.ConfigurationOptions.JiraConfigurationOptions.AutoCreateCases),
		AutoClose:      types.BoolValue(integration.ConfigurationOptions.JiraConfigurationOptions.AutoCloseTickets),
		Credential: &integrationJiraCredentialModel{
			Token: types.StringPointerValue(nil),
		},
	}

	resp.State.Set(ctx, &model)
}
