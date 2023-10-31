// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &IntegrationOciTenantResource{}
var _ resource.ResourceWithImportState = &IntegrationOciTenantResource{}

func NewIntegrationOciTenantResource() resource.Resource {
	return &IntegrationOciTenantResource{}
}

// IntegrationOciTenantResource defines the resource implementation.
type IntegrationOciTenantResource struct {
	client *mondoov1.Client
}

// IntegrationOciTenantResourceModel describes the resource data model.
type IntegrationOciTenantResourceModel struct {
	// scope
	SpaceId types.String `tfsdk:"space_id"`
	OrgId   types.String `tfsdk:"org_id"`

	// integration details
	Mrn        types.String    `tfsdk:"mrn"`
	Name       types.String    `tfsdk:"name"`
	Tenancy    types.String    `tfsdk:"tenancy"`
	Region     types.String    `tfsdk:"region"`
	User       types.String    `tfsdk:"user"`
	Credential CredentialModel `tfsdk:"credentials"`
}

type CredentialModel struct {
	Fingerprint types.String `tfsdk:"fingerprint"`
	PrivateKey  types.String `tfsdk:"private_key"`
}

func (r *IntegrationOciTenantResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_oci_tenant"
}

func (r *IntegrationOciTenantResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Example resource",

		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{ // TODO: add check that either space or org needs to be set
				MarkdownDescription: "Example configurable attribute with default value",
				Optional:            true,
			},
			"org_id": schema.StringAttribute{
				MarkdownDescription: "Example configurable attribute with default value",
				Optional:            true,
			},
			"mrn": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Example identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Example configurable attribute",
				Optional:            true,
			},
			"tenancy": schema.StringAttribute{
				MarkdownDescription: "Example configurable attribute",
				Required:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "Example configurable attribute",
				Required:            true,
			},
			"user": schema.StringAttribute{
				MarkdownDescription: "Example configurable attribute",
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

func (r *IntegrationOciTenantResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	r.client = client
}

func (r *IntegrationOciTenantResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data IntegrationOciTenantResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource.
	var createMutation struct {
		CreateClientIntegration struct {
			Integration struct {
				Mrn  mondoov1.String
				Name mondoov1.String
			}
		} `graphql:"createClientIntegration(input: $input)"`
	}

	spaceMrn := ""
	if data.SpaceId.ValueString() != "" {
		spaceMrn = spacePrefix + data.SpaceId.ValueString()
	}

	createInput := mondoov1.CreateClientIntegrationInput{
		SpaceMrn:       mondoov1.String(spaceMrn),
		Name:           mondoov1.String(data.Name.ValueString()),
		Type:           mondoov1.ClientIntegrationTypeOci,
		LongLivedToken: false,
		ConfigurationOptions: mondoov1.ClientIntegrationConfigurationInput{
			OciConfigurationOptions: &mondoov1.OciConfigurationOptionsInput{
				TenancyOcid: mondoov1.String(data.Tenancy.ValueString()),
				UserOcid:    mondoov1.String(data.User.ValueString()),
				Region:      mondoov1.String(data.Region.ValueString()),
				Fingerprint: mondoov1.String(data.Credential.Fingerprint.ValueString()),
				PrivateKey:  mondoov1.NewStringPtr(mondoov1.String(data.Credential.PrivateKey.ValueString())),
			},
		},
	}

	tflog.Trace(ctx, "CreateSpaceInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", createInput),
	})

	err := r.client.Mutate(context.Background(), &createMutation, createInput, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create OCI tenant integration, got error: %s", err))
		return
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(createMutation.CreateClientIntegration.Integration.Mrn))
	data.Name = types.StringValue(string(createMutation.CreateClientIntegration.Integration.Name))
	data.SpaceId = types.StringValue(data.SpaceId.ValueString())

	// Write logs using the tflog package
	// Documentation: https://terraform.io/plugin/log
	tflog.Trace(ctx, "created a resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IntegrationOciTenantResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IntegrationOciTenantResourceModel

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

func (r *IntegrationOciTenantResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data IntegrationOciTenantResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	var updateMutation struct {
		UpdateClientIntegrationName struct {
			Name mondoov1.String
		} `graphql:"updateClientIntegrationName(input: $input)"`
	}
	updateInput := mondoov1.UpdateClientIntegrationNameInput{
		Mrn:  mondoov1.String(data.Mrn.ValueString()),
		Name: mondoov1.String(data.Name.ValueString()),
	}
	tflog.Trace(ctx, "UpdateClientIntegrationNameInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", updateInput),
	})
	err := r.client.Mutate(context.Background(), &updateMutation, updateInput, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update OCI tenant integration, got error: %s", err))
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IntegrationOciTenantResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data IntegrationOciTenantResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	var deleteMutation struct {
		DeleteClientIntegration struct {
			Mrn mondoov1.String
		} `graphql:"deleteClientIntegration(input: $input)"`
	}
	deleteInput := mondoov1.DeleteClientIntegrationInput{
		Mrn: mondoov1.String(data.Mrn.ValueString()),
	}
	tflog.Trace(ctx, "DeleteClientIntegration", map[string]interface{}{
		"input": fmt.Sprintf("%+v", deleteInput),
	})
	err := r.client.Mutate(context.Background(), &deleteMutation, deleteInput, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete Oci tenant integration, got error: %s", err))
		return
	}
}

func (r *IntegrationOciTenantResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
