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

var _ resource.Resource = (*integrationGcpResource)(nil)

func NewIntegrationGcpResource() resource.Resource {
	return &integrationGcpResource{}
}

type integrationGcpResource struct {
	client *ExtendedGqlClient
}

type integrationGcpResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn       types.String `tfsdk:"mrn"`
	Name      types.String `tfsdk:"name"`
	ProjectID types.String `tfsdk:"project_id"`

	// credentials
	Credential integrationGcpCredentialModel `tfsdk:"credentials"`
}

type integrationGcpCredentialModel struct {
	PrivateKey types.String `tfsdk:"private_key"`
}

func (r *integrationGcpResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_gcp"
}

func (r *integrationGcpResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan Google GCP organizations and projects for misconfigurations and vulnerabilities.`,
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo Space Identifier. If it is not provided, the provider space is used.",
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
			"project_id": schema.StringAttribute{
				MarkdownDescription: "GCP project id",
				Optional:            true,
			},
			"credentials": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"private_key": schema.StringAttribute{
						Required:  true,
						Sensitive: true,
					},
				},
			},
		},
	}
}

func (r *integrationGcpResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *integrationGcpResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationGcpResourceModel

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
		mondoov1.ClientIntegrationTypeGcp,
		mondoov1.ClientIntegrationConfigurationInput{
			GcpConfigurationOptions: &mondoov1.GcpConfigurationOptionsInput{
				ProjectID:      mondoov1.NewStringPtr(mondoov1.String(data.ProjectID.ValueString())),
				ServiceAccount: mondoov1.NewStringPtr(mondoov1.String(data.Credential.PrivateKey.ValueString())),
				DiscoverAll:    mondoov1.NewBooleanPtr(mondoov1.Boolean(true)),
			},
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create GCP integration, got error: %s", err),
			)
		return
	}

	// trigger integration to gather results quickly after the first setup
	// NOTE: we ignore the error since the integration state does not depend on it
	_, err = r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunScan)
	if err != nil {
		resp.Diagnostics.
			AddWarning("Client Error",
				fmt.Sprintf("Unable to trigger integration, got error: %s", err),
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

func (r *integrationGcpResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationGcpResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGcpResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationGcpResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		GcpConfigurationOptions: &mondoov1.GcpConfigurationOptionsInput{
			ProjectID:      mondoov1.NewStringPtr(mondoov1.String(data.ProjectID.ValueString())),
			ServiceAccount: mondoov1.NewStringPtr(mondoov1.String(data.Credential.PrivateKey.ValueString())),
			DiscoverAll:    mondoov1.NewBooleanPtr(mondoov1.Boolean(true)),
		},
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeGcp,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update Gcp integration, got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGcpResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationGcpResourceModel

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
				fmt.Sprintf("Unable to delete Gcp integration, got error: %s", err),
			)
		return
	}
}

func (r *integrationGcpResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {

	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	model := integrationGcpResourceModel{
		Mrn:       types.StringValue(integration.Mrn),
		Name:      types.StringValue(integration.Name),
		SpaceID:   types.StringValue(integration.SpaceID()),
		ProjectID: types.StringValue(integration.ConfigurationOptions.GcpConfigurationOptions.ProjectId),
		Credential: integrationGcpCredentialModel{
			PrivateKey: types.StringPointerValue(nil),
		},
	}

	resp.State.Set(ctx, &model)
}
