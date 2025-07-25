// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1
//
// Code generated by gen.go; DO NOT EDIT.

package provider

import (
	"context"
	"fmt"

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
var _ resource.Resource = (*integrationAzureDevopsResource)(nil)
var _ resource.ResourceWithImportState = (*integrationAzureDevopsResource)(nil)

func NewIntegrationAzureDevopsResource() resource.Resource {
	return &integrationAzureDevopsResource{}
}

type integrationAzureDevopsResource struct {
	client *ExtendedGqlClient
}

type integrationAzureDevopsResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn  types.String `tfsdk:"mrn"`
	Name types.String `tfsdk:"name"`

	// AzureDevops options
	AutoCloseTickets   types.Bool   `tfsdk:"auto_close_tickets"`
	AutoCreateTickets  types.Bool   `tfsdk:"auto_create_tickets"`
	ClientSecret       types.String `tfsdk:"client_secret"`
	DefaultProjectName types.String `tfsdk:"default_project_name"`
	OrganizationUrl    types.String `tfsdk:"organization_url"`
	ServicePrincipalId types.String `tfsdk:"service_principal_id"`
	TenantId           types.String `tfsdk:"tenant_id"`
}

func (m integrationAzureDevopsResourceModel) GetConfigurationOptions() *mondoov1.AzureDevopsConfigurationOptionsInput {
	return &mondoov1.AzureDevopsConfigurationOptionsInput{
		// AzureDevops options
		AutoCloseTickets:   mondoov1.Boolean(m.AutoCloseTickets.ValueBool()),
		AutoCreateTickets:  mondoov1.Boolean(m.AutoCreateTickets.ValueBool()),
		ClientSecret:       mondoov1.String(m.ClientSecret.ValueString()),
		DefaultProjectName: mondoov1.NewStringPtr(mondoov1.String(m.DefaultProjectName.ValueString())),
		OrganizationUrl:    mondoov1.String(m.OrganizationUrl.ValueString()),
		ServicePrincipalId: mondoov1.String(m.ServicePrincipalId.ValueString()),
		TenantId:           mondoov1.String(m.TenantId.ValueString()),
	}
}

func (r *integrationAzureDevopsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_azure_devops"
}

func (r *integrationAzureDevopsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `AzureDevops integration.`,
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
			// AzureDevops options
			"auto_close_tickets": schema.BoolAttribute{
				MarkdownDescription: "The AzureDevops AutoCloseTickets",
				Required:            true,
			},
			"auto_create_tickets": schema.BoolAttribute{
				MarkdownDescription: "The AzureDevops AutoCreateTickets",
				Required:            true,
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "The AzureDevops ClientSecret",
				Required:            true,
			},
			"default_project_name": schema.StringAttribute{
				MarkdownDescription: "The AzureDevops DefaultProjectName",
				Optional:            true,
			},
			"organization_url": schema.StringAttribute{
				MarkdownDescription: "The AzureDevops OrganizationUrl",
				Required:            true,
			},
			"service_principal_id": schema.StringAttribute{
				MarkdownDescription: "The AzureDevops ServicePrincipalId",
				Required:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "The AzureDevops TenantId",
				Required:            true,
			},
		},
	}
}

func (r *integrationAzureDevopsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)

	if !ok {
		resp.
			Diagnostics.
			AddError("Unexpected Resource Configure Type",
				fmt.Sprintf(
					"Expected *http.Client. Got: %T. Please report this issue to the provider developers.",
					req.ProviderData,
				),
			)
		return
	}

	r.client = client
}

func (r *integrationAzureDevopsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

	var data integrationAzureDevopsResourceModel

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
		mondoov1.ClientIntegrationTypeTicketSystemAzureDevops,
		mondoov1.ClientIntegrationConfigurationInput{
			AzureDevopsConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.
			Diagnostics.
			AddError("Client Error",
				fmt.Sprintf(
					"Unable to create %s integration. Got error: %s", mondoov1.IntegrationTypeTicketSystemAzureDevops, err,
				),
			)
		return
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(data.Name.ValueString())
	data.SpaceID = types.StringValue(space.ID())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAzureDevopsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationAzureDevopsResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAzureDevopsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationAzureDevopsResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		AzureDevopsConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeTicketSystemAzureDevops,
		opts,
	)
	if err != nil {
		resp.
			Diagnostics.
			AddError("Client Error",
				fmt.Sprintf(
					"Unable to update %s integration. Got error: %s", mondoov1.IntegrationTypeTicketSystemAzureDevops, err,
				),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAzureDevopsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationAzureDevopsResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.
			Diagnostics.
			AddError("Client Error",
				fmt.Sprintf(
					"Unable to delete %s integration. Got error: %s", mondoov1.IntegrationTypeTicketSystemAzureDevops, err,
				),
			)
		return
	}
}

func (r *integrationAzureDevopsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}
	model := integrationAzureDevopsResourceModel{
		Mrn:     types.StringValue(integration.Mrn),
		Name:    types.StringValue(integration.Name),
		SpaceID: types.StringValue(integration.SpaceID()),
		// AzureDevops options
		AutoCloseTickets:   types.BoolValue(integration.ConfigurationOptions.AzureDevopsConfigurationOptions.AutoCloseTickets),
		AutoCreateTickets:  types.BoolValue(integration.ConfigurationOptions.AzureDevopsConfigurationOptions.AutoCreateTickets),
		ClientSecret:       types.StringValue(""),
		DefaultProjectName: types.StringPointerValue(integration.ConfigurationOptions.AzureDevopsConfigurationOptions.DefaultProjectName),
		OrganizationUrl:    types.StringValue(integration.ConfigurationOptions.AzureDevopsConfigurationOptions.OrganizationUrl),
		ServicePrincipalId: types.StringValue(integration.ConfigurationOptions.AzureDevopsConfigurationOptions.ServicePrincipalId),
		TenantId:           types.StringValue(integration.ConfigurationOptions.AzureDevopsConfigurationOptions.TenantId),
	}

	resp.State.Set(ctx, &model)
}

// AzureDevops options for import state
type AzureDevopsConfigurationOptions struct {
	AutoCloseTickets   bool    `json:"auto_close_tickets"`
	AutoCreateTickets  bool    `json:"auto_create_tickets"`
	DefaultProjectName *string `json:"default_project_name"`
	OrganizationUrl    string  `json:"organization_url"`
	ServicePrincipalId string  `json:"service_principal_id"`
	TenantId           string  `json:"tenant_id"`
}
