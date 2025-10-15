// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = (*integrationEmailResource)(nil)
var _ resource.ResourceWithImportState = (*integrationEmailResource)(nil)

func NewIntegrationEmailResource() resource.Resource {
	return &integrationEmailResource{}
}

type integrationEmailResource struct {
	client *ExtendedGqlClient
}

type integrationEmailResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn               types.String                      `tfsdk:"mrn"`
	Name              types.String                      `tfsdk:"name"`
	Recipients        *[]integrationEmailRecipientInput `tfsdk:"recipients"`
	AutoCreateTickets types.Bool                        `tfsdk:"auto_create"`
	AutoCloseTickets  types.Bool                        `tfsdk:"auto_close"`
}

type integrationEmailRecipientInput struct {
	Name         types.String `tfsdk:"name"`
	Email        types.String `tfsdk:"email"`
	IsDefault    types.Bool   `tfsdk:"is_default"`
	ReferenceURL types.String `tfsdk:"reference_url"`
}

func (m integrationEmailResourceModel) GetConfigurationOptions() *mondoov1.EmailConfigurationOptionsInput {
	opts := &mondoov1.EmailConfigurationOptionsInput{
		Recipients:        convertRecipients(m.Recipients),
		AutoCreateTickets: mondoov1.NewBooleanPtr(mondoov1.Boolean(m.AutoCreateTickets.ValueBool())),
		AutoCloseTickets:  mondoov1.NewBooleanPtr(mondoov1.Boolean(m.AutoCloseTickets.ValueBool())),
	}

	return opts
}

func (r *integrationEmailResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_email"
}

type defaultRecipientValidator struct{}

func (d defaultRecipientValidator) ValidateList(ctx context.Context, req validator.ListRequest, resp *validator.ListResponse) {
	// Return early if value is null or unknown
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	// Convert ListValue to a slice of ObjectValues
	var recipients []basetypes.ObjectValue
	diags := req.ConfigValue.ElementsAs(ctx, &recipients, false)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	defaultCount := 0

	// Iterate over recipients to count default ones
	for _, recipient := range recipients {
		// Access the attributes of the recipient
		attrs := recipient.Attributes()

		// Retrieve the "is_default" attribute
		isDefaultAttr, exists := attrs["is_default"]
		if !exists {
			resp.Diagnostics.AddError(
				"Missing Attribute",
				"Recipient object is missing the 'is_default' attribute.",
			)
			return
		}

		// Check if the value is true
		isDefault, ok := isDefaultAttr.(types.Bool)
		if !ok {
			resp.Diagnostics.AddError(
				"Invalid Attribute Type",
				"The 'is_default' attribute must be a boolean.",
			)
			return
		}

		if isDefault.ValueBool() {
			defaultCount++
		}
	}

	// Validate that only one recipient is marked as default
	if defaultCount > 1 {
		resp.Diagnostics.AddError(
			"Too Many Default Recipients",
			"Only one recipient can be marked as default.",
		)
	}
}

func (d defaultRecipientValidator) Description(ctx context.Context) string {
	return "Ensures that only one recipient is marked as default."
}

func (d defaultRecipientValidator) MarkdownDescription(ctx context.Context) string {
	return "Ensures that only one recipient is marked as `default`."
}

func NewDefaultRecipientValidator() validator.List {
	return defaultRecipientValidator{}
}

type AutoCreateValidator struct{}

func (v AutoCreateValidator) ValidateBool(ctx context.Context, req validator.BoolRequest, resp *validator.BoolResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}

	autoCreate := req.ConfigValue.ValueBool()
	if !autoCreate {
		return
	}

	// Retrieve the recipients list from configuration
	var recipientsAttr basetypes.ListValue
	recipientsDiags := req.Config.GetAttribute(ctx, path.Root("recipients"), &recipientsAttr)
	if recipientsDiags.HasError() {
		resp.Diagnostics.Append(recipientsDiags...)
		return
	}

	var recipients []basetypes.ObjectValue
	diags := recipientsAttr.ElementsAs(ctx, &recipients, false)
	if diags.HasError() {
		resp.Diagnostics.Append(diags...)
		return
	}

	defaultFound := false
	for _, recipient := range recipients {
		attrs := recipient.Attributes()

		isDefaultAttr, exists := attrs["is_default"]
		if exists {
			isDefault, ok := isDefaultAttr.(types.Bool)
			if !ok {
				resp.Diagnostics.AddError(
					"Invalid Attribute Type",
					"The 'is_default' attribute must be a boolean.",
				)
				return
			}
			if isDefault.ValueBool() {
				defaultFound = true
				break
			}
		}
	}

	if !defaultFound {
		resp.Diagnostics.AddError(
			"Missing Default Recipient",
			"At least one recipient must be marked as default when auto-create is enabled.",
		)
	}
}

func (v AutoCreateValidator) Description(ctx context.Context) string {
	return "Ensures that at least one recipient is marked as default when auto-create is enabled."
}

func (v AutoCreateValidator) MarkdownDescription(ctx context.Context) string {
	return "Ensures that at least one recipient is marked as `default` when auto-create is enabled."
}

func NewAutoCreateValidator() validator.Bool {
	return AutoCreateValidator{}
}

func (r *integrationEmailResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Send email to your ticket system or any recipient.",
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
			"recipients": schema.ListNestedAttribute{
				MarkdownDescription: "List of email recipients.",
				Required:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							MarkdownDescription: "Recipient name.",
							Required:            true,
						},
						"email": schema.StringAttribute{
							MarkdownDescription: "Recipient email address.",
							Required:            true,
							Validators: []validator.String{
								stringvalidator.RegexMatches(
									regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
									"must be a valid email",
								),
							},
						},
						"is_default": schema.BoolAttribute{
							MarkdownDescription: "Mark this recipient as default. This must be set if auto_create is enabled.",
							Optional:            true,
						},
						"reference_url": schema.StringAttribute{
							MarkdownDescription: "Optional reference URL for the recipient.",
							Optional:            true,
						},
					},
				},
				Validators: []validator.List{
					NewDefaultRecipientValidator(),
				},
			},
			"auto_create": schema.BoolAttribute{
				MarkdownDescription: "Auto create tickets (defaults to false).",
				Optional:            true,
				Validators: []validator.Bool{
					NewAutoCreateValidator(),
				},
			},
			"auto_close": schema.BoolAttribute{
				MarkdownDescription: "Auto close tickets (defaults to false).",
				Optional:            true,
			},
		},
	}
}

func (r *integrationEmailResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func convertRecipients(recipients *[]integrationEmailRecipientInput) []mondoov1.EmailRecipientInput {
	if recipients == nil {
		return nil
	}
	var result []mondoov1.EmailRecipientInput
	for _, r := range *recipients {
		result = append(result, mondoov1.EmailRecipientInput{
			Name:         mondoov1.String(r.Name.ValueString()),
			Email:        mondoov1.String(r.Email.ValueString()),
			IsDefault:    mondoov1.Boolean(r.IsDefault.ValueBool()),
			ReferenceUrl: mondoov1.NewStringPtr(mondoov1.String(r.ReferenceURL.ValueString())),
		})
	}
	return result
}

func (r *integrationEmailResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationEmailResourceModel

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
		mondoov1.ClientIntegrationTypeTicketSystemEmail,
		mondoov1.ClientIntegrationConfigurationInput{
			EmailConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create email integration. Got error: %s", err),
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

func (r *integrationEmailResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationEmailResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationEmailResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationEmailResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		EmailConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeTicketSystemEmail,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update email integration. Got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationEmailResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationEmailResourceModel

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
				fmt.Sprintf("Unable to delete email integration. Got error: %s", err),
			)
		return
	}
}

func (r *integrationEmailResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	var recipients []integrationEmailRecipientInput
	for _, recipient := range integration.ConfigurationOptions.EmailConfigurationOptions.Recipients {
		recipients = append(recipients, integrationEmailRecipientInput{
			Name:         types.StringValue(recipient.Name),
			Email:        types.StringValue(recipient.Email),
			IsDefault:    types.BoolValue(recipient.IsDefault),
			ReferenceURL: types.StringValue(recipient.ReferenceURL),
		})
	}

	model := integrationEmailResourceModel{
		Mrn:               types.StringValue(integration.Mrn),
		Name:              types.StringValue(integration.Name),
		SpaceID:           types.StringValue(integration.SpaceID()),
		AutoCreateTickets: types.BoolValue(integration.ConfigurationOptions.EmailConfigurationOptions.AutoCreateTickets),
		AutoCloseTickets:  types.BoolValue(integration.ConfigurationOptions.EmailConfigurationOptions.AutoCloseTickets),
		Recipients:        &recipients,
	}

	resp.State.Set(ctx, &model)
}
