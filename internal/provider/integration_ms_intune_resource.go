// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
var _ resource.Resource = (*integrationMsIntuneResource)(nil)
var _ resource.ResourceWithImportState = (*integrationMsIntuneResource)(nil)

func NewIntegrationMsIntuneResource() resource.Resource {
	return &integrationMsIntuneResource{}
}

type integrationMsIntuneResource struct {
	client *ExtendedGqlClient
}

type integrationMsIntuneResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn      types.String `tfsdk:"mrn"`
	Name     types.String `tfsdk:"name"`
	ClientId types.String `tfsdk:"client_id"`
	TenantId types.String `tfsdk:"tenant_id"`

	// options
	ImportDevices types.Bool `tfsdk:"import_devices"`
	AiDiscovery   types.Bool `tfsdk:"ai_discovery"`

	// credentials
	Credential integrationMsIntuneCredentialModel `tfsdk:"credentials"`
}

type integrationMsIntuneCredentialModel struct {
	ClientSecret types.String `tfsdk:"client_secret"`
}

func (m integrationMsIntuneResourceModel) GetConfigurationOptions() *mondoov1.MsIntuneConfigurationOptionsInput {
	opts := &mondoov1.MsIntuneConfigurationOptionsInput{
		TenantId: mondoov1.String(m.TenantId.ValueString()),
		ClientId: mondoov1.String(m.ClientId.ValueString()),
		// Always sent: the attributes default to false, so Terraform owns their
		// value and turning one off must reach the API as an explicit false.
		ImportDevices: mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ImportDevices.ValueBool())),
		AiDiscovery:   mondoov1.NewBooleanPtr(mondoov1.Boolean(m.AiDiscovery.ValueBool())),
	}

	if secret := m.Credential.ClientSecret.ValueString(); secret != "" {
		opts.Password = mondoov1.NewStringPtr(mondoov1.String(secret))
	}

	return opts
}

func (r *integrationMsIntuneResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_ms_intune"
}

func (r *integrationMsIntuneResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan and remediate your Microsoft Intune managed devices. To learn more, read the [Mondoo documentation](https://mondoo.com/docs/platform/infra/saas/ms365/ms365-auto/).`,
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
			"client_id": schema.StringAttribute{
				MarkdownDescription: "Azure client ID.",
				Required:            true,
			},
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Azure tenant ID.",
				Required:            true,
			},
			"import_devices": schema.BoolAttribute{
				MarkdownDescription: "Import the Intune-managed Windows devices and their detected software as assets. A device that is also scanned by cnspec is matched to its existing asset instead of creating a new one. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"ai_discovery": schema.BoolAttribute{
				MarkdownDescription: "Discover MCP servers configured on each endpoint during fleet scans (as their own assets). Enabling it also activates the Mondoo AI Security policy in the integration's space. Discovering a stdio MCP server starts the command in its configuration file, with the privileges the scan runs with. Defaults to `false`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(false),
			},
			"credentials": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"client_secret": schema.StringAttribute{
						MarkdownDescription: "Client secret for the Intune integration.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
		},
	}
}

func (r *integrationMsIntuneResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ExtendedGqlClient. Got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *integrationMsIntuneResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationMsIntuneResourceModel

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
		mondoov1.ClientIntegrationTypeMsIntune,
		mondoov1.ClientIntegrationConfigurationInput{
			MsIntuneConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create MS Intune integration. Got error: %s", err),
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

func (r *integrationMsIntuneResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationMsIntuneResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Refresh the attributes the API reports back.
	if data.Mrn.ValueString() != "" {
		integration, err := r.client.GetClientIntegration(ctx, data.Mrn.ValueString())
		if err != nil {
			// Only drop the resource from state when it genuinely no longer
			// exists. A transient error must not remove it from state.
			if isNotFoundError(err) {
				resp.State.RemoveResource(ctx)
				return
			}
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to read MS Intune integration: %s", err),
			)
			return
		}
		opts := integration.ConfigurationOptions.MsIntuneConfigurationOptions
		data.ImportDevices = types.BoolValue(opts.ImportDevices)
		data.AiDiscovery = types.BoolValue(opts.AiDiscovery)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationMsIntuneResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationMsIntuneResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		MsIntuneConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeMsIntune,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update MS Intune integration. Got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationMsIntuneResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationMsIntuneResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to delete the resource.
	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to delete MS Intune integration. Got error: %s", err),
			)
		return
	}
}

func (r *integrationMsIntuneResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	opts := integration.ConfigurationOptions.MsIntuneConfigurationOptions
	if opts == (MsIntuneConfigurationOptions{}) {
		resp.Diagnostics.AddError("Import Error", "Integration is not an MS Intune integration")
		return
	}

	model := integrationMsIntuneResourceModel{
		Mrn:      types.StringValue(integration.Mrn),
		Name:     types.StringValue(integration.Name),
		SpaceID:  types.StringValue(integration.SpaceID()),
		TenantId: types.StringValue(opts.TenantId),
		ClientId: types.StringValue(opts.ClientId),
		// options
		ImportDevices: types.BoolValue(opts.ImportDevices),
		AiDiscovery:   types.BoolValue(opts.AiDiscovery),
		Credential: integrationMsIntuneCredentialModel{
			ClientSecret: types.StringPointerValue(nil),
		},
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
