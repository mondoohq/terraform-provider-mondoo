// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"strings"

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
var _ resource.Resource = &integrationOciTenantResource{}
var _ resource.ResourceWithImportState = &integrationOciTenantResource{}

func NewIntegrationOciTenantResource() resource.Resource {
	return &integrationOciTenantResource{}
}

// integrationOciTenantResource defines the resource implementation.
type integrationOciTenantResource struct {
	client *ExtendedGqlClient
}

// integrationOciTenantResourceModel describes the resource data model.
type integrationOciTenantResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn     types.String `tfsdk:"mrn"`
	Name    types.String `tfsdk:"name"`
	Tenancy types.String `tfsdk:"tenancy"`
	Region  types.String `tfsdk:"region"`
	User    types.String `tfsdk:"user"`

	// credentials
	Credential integrationOciCredentialModel `tfsdk:"credentials"`
}

type integrationOciCredentialModel struct {
	Fingerprint types.String `tfsdk:"fingerprint"`
	PrivateKey  types.String `tfsdk:"private_key"`
}

func (r *integrationOciTenantResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_oci_tenant"
}

func (r *integrationOciTenantResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Example resource",

		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo Space Identifier. If it is not provided, the provider space is used.",
				Optional:            true,
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
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"tenancy": schema.StringAttribute{
				MarkdownDescription: "OCI tenancy",
				Required:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "OCI region",
				Required:            true,
			},
			"user": schema.StringAttribute{
				MarkdownDescription: "OCI user",
				Required:            true,
			},
			"credentials": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"fingerprint": schema.StringAttribute{
						Required: true,
					},
					"private_key": schema.StringAttribute{
						Required:  true,
						Sensitive: true,
					},
				},
			},
		},
	}
}

func (r *integrationOciTenantResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationOciTenantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationOciTenantResourceModel

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
		mondoov1.ClientIntegrationTypeOci,
		mondoov1.ClientIntegrationConfigurationInput{
			OciConfigurationOptions: &mondoov1.OciConfigurationOptionsInput{
				TenancyOcid: mondoov1.String(data.Tenancy.ValueString()),
				UserOcid:    mondoov1.String(data.User.ValueString()),
				Region:      mondoov1.String(data.Region.ValueString()),
				Fingerprint: mondoov1.String(data.Credential.Fingerprint.ValueString()),
				PrivateKey:  mondoov1.NewStringPtr(mondoov1.String(data.Credential.PrivateKey.ValueString())),
			},
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create OCI tenant integration, got error: %s", err),
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

func (r *integrationOciTenantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationOciTenantResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Write logs using the tflog package
	tflog.Trace(ctx, "read a OCI integration resource")

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationOciTenantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationOciTenantResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	opts := mondoov1.ClientIntegrationConfigurationInput{
		OciConfigurationOptions: &mondoov1.OciConfigurationOptionsInput{
			TenancyOcid: mondoov1.String(data.Tenancy.ValueString()),
			UserOcid:    mondoov1.String(data.User.ValueString()),
			Region:      mondoov1.String(data.Region.ValueString()),
			Fingerprint: mondoov1.String(data.Credential.Fingerprint.ValueString()),
			PrivateKey:  mondoov1.NewStringPtr(mondoov1.String(data.Credential.PrivateKey.ValueString())),
		},
	}

	// Do GraphQL request to API to update the resource.
	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeOci,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update OCI tenant integration, got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationOciTenantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationOciTenantResourceModel

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
				fmt.Sprintf("Unable to delete Oci tenant integration, got error: %s", err),
			)
		return
	}
}

func (r *integrationOciTenantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	mrn := req.ID
	integration, err := r.client.GetClientIntegration(ctx, mrn)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to get integration, got error: %s", err),
			)
		return
	}

	spaceID := strings.Split(integration.Mrn, "/")[len(strings.Split(integration.Mrn, "/"))-3]
	if r.client.Space().ID() != "" && r.client.Space().ID() != spaceID {
		// The provider is configured to manage resources in a different space than the one the
		// resource is currently configured, we won't allow that
		resp.Diagnostics.AddError(
			"Conflict Error",
			fmt.Sprintf(
				"Unable to import integration, the provider is configured in a different space than the resource. (%s != %s)",
				r.client.Space().ID(), spaceID),
		)
		return
	}

	model := integrationOciTenantResourceModel{
		Mrn:     types.StringValue(integration.Mrn),
		Name:    types.StringValue(integration.Name),
		SpaceID: types.StringValue(spaceID),
	}

	resp.State.Set(ctx, &model)
}
