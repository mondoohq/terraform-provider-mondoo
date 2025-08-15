// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

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
var _ resource.Resource = (*integrationSentinelOneResource)(nil)
var _ resource.ResourceWithImportState = (*integrationSentinelOneResource)(nil)

func NewIntegrationSentinelOneResource() resource.Resource {
	return &integrationSentinelOneResource{}
}

type integrationSentinelOneResource struct {
	client *ExtendedGqlClient
}

type integrationSentinelOneResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn  types.String `tfsdk:"mrn"`
	Name types.String `tfsdk:"name"`

	// SentinelOne options
	Host       types.String                          `tfsdk:"host"`
	Account    types.String                          `tfsdk:"account"`
	Credential integrationSentinelOneCredentialModel `tfsdk:"credentials"`
}

type integrationSentinelOneCredentialModel struct {
	Certificate  types.String `tfsdk:"certificate"`
	ClientSecret types.String `tfsdk:"client_secret"`
}

func (m integrationSentinelOneResourceModel) GetConfigurationOptions() *mondoov1.SentinelOneConfigurationOptionsInput {
	// SentinelOne options
	opts := &mondoov1.SentinelOneConfigurationOptionsInput{
		Host:    mondoov1.String(m.Host.ValueString()),
		Account: mondoov1.String(m.Account.ValueString()),
	}

	if certificate := m.Credential.Certificate.ValueString(); certificate != "" {
		opts.Certificate = mondoov1.NewStringPtr(mondoov1.String(certificate))
	}

	if secret := m.Credential.ClientSecret.ValueString(); secret != "" {
		opts.ClientSecret = mondoov1.NewStringPtr(mondoov1.String(secret))
	}

	return opts
}

func (r *integrationSentinelOneResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_sentinel_one"
}

func (r *integrationSentinelOneResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `SentinelOne integration.`,
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
			// SentinelOne options
			"host": schema.StringAttribute{
				MarkdownDescription: "The host of the SentinelOne integration.",
				Required:            true,
			},
			"account": schema.StringAttribute{
				MarkdownDescription: "The account ID of the SentinelOne integration.",
				Required:            true,
			},
			"credentials": schema.SingleNestedAttribute{
				Required:            true,
				MarkdownDescription: "Credentials require one of certificate or client secret to be provided.",
				Attributes: map[string]schema.Attribute{
					"certificate": schema.StringAttribute{
						MarkdownDescription: "The certificate for the SentinelOne integration.",
						Optional:            true,
						Sensitive:           true,
					},
					"client_secret": schema.StringAttribute{
						MarkdownDescription: "The client secret of the SentinelOne integration.",
						Optional:            true,
						Sensitive:           true,
					},
				},
			},
		},
	}
}

func (r *integrationSentinelOneResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationSentinelOneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationSentinelOneResourceModel

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
		mondoov1.ClientIntegrationTypeSentinelOne,
		mondoov1.ClientIntegrationConfigurationInput{
			SentinelOneConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.
			Diagnostics.
			AddError("Client Error",
				fmt.Sprintf(
					"Unable to create %s integration. Got error: %s", mondoov1.IntegrationTypeSentinelOne, err,
				),
			)
		return
	}

	// trigger integration to gather results quickly after the first setup
	// NOTE: we ignore the error since the integration state does not depend on it
	_, err = r.client.TriggerAction(ctx,
		string(integration.Mrn),
		mondoov1.ActionTypeRunImport,
	)
	if err != nil {
		resp.
			Diagnostics.
			AddWarning("Client Error",
				fmt.Sprintf(
					"Unable to trigger integration. Got error: %s", err,
				),
			)
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(data.Name.ValueString())
	data.SpaceID = types.StringValue(space.ID())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationSentinelOneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationSentinelOneResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationSentinelOneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationSentinelOneResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		SentinelOneConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeSentinelOne,
		opts,
	)
	if err != nil {
		resp.
			Diagnostics.
			AddError("Client Error",
				fmt.Sprintf(
					"Unable to update %s integration. Got error: %s", mondoov1.IntegrationTypeSentinelOne, err,
				),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationSentinelOneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationSentinelOneResourceModel

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
					"Unable to delete %s integration. Got error: %s", mondoov1.IntegrationTypeSentinelOne, err,
				),
			)
		return
	}
}

func (r *integrationSentinelOneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}
	model := integrationSentinelOneResourceModel{
		Mrn:     types.StringValue(integration.Mrn),
		Name:    types.StringValue(integration.Name),
		SpaceID: types.StringValue(integration.SpaceID()),
		// SentinelOne options
		Host:    types.StringValue(integration.ConfigurationOptions.SentinelOneConfigurationOptions.Host),
		Account: types.StringValue(integration.ConfigurationOptions.SentinelOneConfigurationOptions.Account),
		Credential: integrationSentinelOneCredentialModel{
			Certificate:  types.StringPointerValue(nil),
			ClientSecret: types.StringPointerValue(nil),
		},
	}

	resp.State.Set(ctx, &model)
}
