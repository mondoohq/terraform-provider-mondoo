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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = (*integrationDomainResource)(nil)
var _ resource.ResourceWithImportState = &integrationDomainResource{}

func NewIntegrationDomainResource() resource.Resource {
	return &integrationDomainResource{}
}

type integrationDomainResource struct {
	client *ExtendedGqlClient
}

type integrationDomainResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn   types.String `tfsdk:"mrn"`
	Host  types.String `tfsdk:"host"`  // full domain name or IP address
	Https types.Bool   `tfsdk:"https"` // https port - default is true
	Http  types.Bool   `tfsdk:"http"`  // http port
}

func (r *integrationDomainResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_domain"
}

// OneRequiredValidator ensures at only one of two boolean attributes is set to true.
type OneRequiredValidator struct {
	OtherAttribute string
}

// ValidateBool performs the validation for the boolean attribute.
func (v OneRequiredValidator) ValidateBool(ctx context.Context, req validator.BoolRequest, resp *validator.BoolResponse) {
	// Retrieve the other attribute's value
	var otherAttr types.Bool
	diags := req.Config.GetAttribute(ctx, path.Root(v.OtherAttribute), &otherAttr)
	// Check if at least one of the attributes is set to true
	if diags.HasError() || !req.ConfigValue.ValueBool() && !otherAttr.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"At Least One Required",
			"Either 'http' or 'https' must be set to true.",
		)
	}
	// Check if both attributes are set to true
	if req.ConfigValue.ValueBool() && otherAttr.ValueBool() {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"Invalid Attribute Combination",
			"Only one of 'http' or 'https' can be set to true.",
		)
	}
}

// Description returns a plain-text description of the validator's purpose.
func (v OneRequiredValidator) Description(ctx context.Context) string {
	return "Ensures that only one of 'http' or 'https' is set to true."
}

// MarkdownDescription returns a markdown-formatted description of the validator's purpose.
func (v OneRequiredValidator) MarkdownDescription(ctx context.Context) string {
	return "Ensures that only one of `http` or `https` is set to `true`."
}

// NewOneRequiredValidator is a convenience function to create an instance of the validator.
func NewOneRequiredValidator(otherAttribute string) validator.Bool {
	return &OneRequiredValidator{
		OtherAttribute: otherAttribute,
	}
}

func (r *integrationDomainResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan endpoints to evaluate domain TLS, SSL, HTTP, and HTTPS security`,
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
			"host": schema.StringAttribute{
				MarkdownDescription: "Domain name or IP address.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^((25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])\.){3}(25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])$|^([a-z0-9-]+\.)+[a-z]{2,}$`),
						"must contain only lowercase letters and at least one dot or be an IPv4 address",
					),
				},
			},
			"https": schema.BoolAttribute{
				MarkdownDescription: "Enable HTTPS port.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				Validators: []validator.Bool{
					NewOneRequiredValidator("http"),
				},
			},
			"http": schema.BoolAttribute{
				MarkdownDescription: "Enable HTTP port.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
				Validators: []validator.Bool{
					NewOneRequiredValidator("https"),
				},
			},
		},
	}
}

func (r *integrationDomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

	var data integrationDomainResourceModel

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
		data.Host.ValueString(),
		mondoov1.ClientIntegrationTypeHost,
		mondoov1.ClientIntegrationConfigurationInput{
			HostConfigurationOptions: &mondoov1.HostConfigurationOptionsInput{
				Host:  mondoov1.String(data.Host.ValueString()),
				Https: mondoov1.NewBooleanPtr(mondoov1.Boolean(data.Https.ValueBool())),
				Http:  mondoov1.NewBooleanPtr(mondoov1.Boolean(data.Http.ValueBool())),
			},
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create domain integration. Got error: %s", err),
			)
		return
	}

	// trigger integration to gather results quickly after the first setup
	// NOTE: we ignore the error since the integration state does not depend on it
	_, err = r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunScan)
	if err != nil {
		resp.Diagnostics.
			AddWarning("Client Error",
				fmt.Sprintf("Unable to trigger integration. Got error: %s", err),
			)
		return
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Host = types.StringValue(data.Host.ValueString())
	data.SpaceID = types.StringValue(space.ID())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationDomainResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic
	integration, err := r.client.GetClientIntegration(ctx, data.Mrn.ValueString())
	fmt.Println("Read integration domain mrn:", data.Mrn.ValueString())
	fmt.Println("Read Error:", err)
	if err != nil {
		resp.State.RemoveResource(ctx)
		return
	}

	model := integrationDomainResourceModel{
		SpaceID: types.StringValue(integration.SpaceID()),
		Mrn:     types.StringValue(integration.Mrn),
		Host:    types.StringValue(integration.ConfigurationOptions.HostConfigurationOptions.Host),
		Https:   types.BoolValue(integration.ConfigurationOptions.HostConfigurationOptions.HTTPS),
		Http:    types.BoolValue(integration.ConfigurationOptions.HostConfigurationOptions.HTTP),
	}

	fmt.Println("Read integration domain resource:", model)

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *integrationDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationDomainResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		HostConfigurationOptions: &mondoov1.HostConfigurationOptionsInput{
			Host:  mondoov1.String(data.Host.ValueString()),
			Https: mondoov1.NewBooleanPtr(mondoov1.Boolean(data.Https.ValueBool())),
			Http:  mondoov1.NewBooleanPtr(mondoov1.Boolean(data.Http.ValueBool())),
		},
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Host.ValueString(),
		mondoov1.ClientIntegrationTypeHost,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update domain integration. Got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationDomainResourceModel

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
				fmt.Sprintf("Unable to delete domain integration. Got error: %s", err),
			)
		return
	}
}

func (r *integrationDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	model := integrationDomainResourceModel{
		SpaceID: types.StringValue(integration.SpaceID()),
		Mrn:     types.StringValue(integration.Mrn),
		Host:    types.StringValue(integration.ConfigurationOptions.HostConfigurationOptions.Host),
		Https:   types.BoolValue(integration.ConfigurationOptions.HostConfigurationOptions.HTTPS),
		Http:    types.BoolValue(integration.ConfigurationOptions.HostConfigurationOptions.HTTP),
	}

	resp.State.Set(ctx, &model)
}
