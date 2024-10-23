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

var _ resource.Resource = (*integrationShodanResource)(nil)

func NewIntegrationShodanResource() resource.Resource {
	return &integrationShodanResource{}
}

type integrationShodanResource struct {
	client *ExtendedGqlClient
}

type integrationShodanResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn  types.String `tfsdk:"mrn"`
	Name types.String `tfsdk:"name"`

	// Shodan scan targets
	Targets types.List `tfsdk:"targets"`

	// credentials
	Credentials *integrationShodanCredentialModel `tfsdk:"credentials"`
}

type integrationShodanCredentialModel struct {
	Token types.String `tfsdk:"token"`
}

func (r *integrationShodanResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_shodan"
}

func (r *integrationShodanResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously assess external risk for domains and IP addresses.`,
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo Space Identifier. If it is not provided, the provider space is used.",
				Optional:            true,
				Computed:            true,
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
			"targets": schema.ListAttribute{
				MarkdownDescription: "Shodan scan targets.",
				Required:            true,
				ElementType:         types.StringType,
			},
			"credentials": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"token": schema.StringAttribute{
						MarkdownDescription: "Token for Shodan integration.",
						Required:            true,
						Sensitive:           true,
						Validators: []validator.String{
							stringvalidator.LengthAtLeast(10),
						},
					},
				},
			},
		},
	}
}

func (r *integrationShodanResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
					"Expected *http.Client, got: %T. Please report this issue to the provider developers.",
					req.ProviderData,
				),
			)
		return
	}

	r.client = client
}

func (r *integrationShodanResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

	var data integrationShodanResourceModel

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
	targets := ConvertSliceStrings(data.Targets)
	ctx = tflog.SetField(ctx, "targets", targets)

	tflog.Debug(ctx, "Creating integration")
	integration, err := r.client.CreateIntegration(ctx,
		space.MRN(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeShodan,
		mondoov1.ClientIntegrationConfigurationInput{
			ShodanConfigurationOptions: &mondoov1.ShodanConfigurationOptionsInput{
				Targets: &targets,
				Token:   mondoov1.String(data.Credentials.Token.ValueString()),
			},
		})
	if err != nil {
		resp.
			Diagnostics.
			AddError("Client Error",
				fmt.Sprintf(
					"Unable to create integration, got error: %s", err,
				),
			)
		return
	}

	// trigger integration to gather results quickly after the first setup
	_, err = r.client.TriggerAction(ctx,
		string(integration.Mrn),
		mondoov1.ActionTypeRunScan,
	)
	if err != nil {
		resp.
			Diagnostics.
			AddWarning("Client Error",
				fmt.Sprintf(
					"Unable to trigger integration, got error: %s", err,
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

func (r *integrationShodanResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationShodanResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationShodanResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationShodanResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	targets := ConvertSliceStrings(data.Targets)
	opts := mondoov1.ClientIntegrationConfigurationInput{
		ShodanConfigurationOptions: &mondoov1.ShodanConfigurationOptionsInput{
			Targets: &targets,
			Token:   mondoov1.String(data.Credentials.Token.ValueString()),
		},
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeShodan,
		opts,
	)
	if err != nil {
		resp.
			Diagnostics.
			AddError("Client Error",
				fmt.Sprintf(
					"Unable to update Domain integration, got error: %s", err,
				),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationShodanResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationShodanResourceModel

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
					"Unable to delete Domain integration, got error: %s", err,
				),
			)
		return
	}
}

func (r *integrationShodanResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
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

	model := integrationShodanResourceModel{
		Mrn:     types.StringValue(mrn),
		Name:    types.StringValue(integration.Name),
		SpaceID: types.StringValue(spaceID),
		Targets: ConvertListValue(
			integration.ConfigurationOptions.ShodanConfigurationOptions.Targets,
		),
	}

	resp.State.Set(ctx, &model)
}
