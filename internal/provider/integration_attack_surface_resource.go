// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var _ resource.Resource = (*integrationAttackSurfaceResource)(nil)
var _ resource.ResourceWithImportState = (*integrationAttackSurfaceResource)(nil)

func NewIntegrationAttackSurfaceResource() resource.Resource {
	return &integrationAttackSurfaceResource{}
}

type integrationAttackSurfaceResource struct {
	client *ExtendedGqlClient
}

type integrationAttackSurfaceResourceModel struct {
	SpaceID types.String `tfsdk:"space_id"`
	Mrn     types.String `tfsdk:"mrn"`
	Name    types.String `tfsdk:"name"`

	Domains  types.List   `tfsdk:"domains"`
	Schedule types.String `tfsdk:"schedule"`

	Subdomains            types.Bool `tfsdk:"subdomains"`
	PortScan              types.Bool `tfsdk:"port_scan"`
	WebFingerprint        types.Bool `tfsdk:"web_fingerprint"`
	VulnerabilityScanning types.Bool `tfsdk:"vulnerability_scanning"`
	SubdomainTakeover     types.Bool `tfsdk:"subdomain_takeover"`
	CloudDiscovery        types.Bool `tfsdk:"cloud_discovery"`
	Webscan               types.Bool `tfsdk:"webscan"`
	PauseScanning         types.Bool `tfsdk:"pause_scanning"`
}

func (r *integrationAttackSurfaceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_attack_surface"
}

func optionalBool(description string, def bool) schema.BoolAttribute {
	return schema.BoolAttribute{
		MarkdownDescription: fmt.Sprintf("%s Defaults to `%t`.", description, def),
		Optional:            true,
		Computed:            true,
		Default:             booldefault.StaticBool(def),
	}
}

func (r *integrationAttackSurfaceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Continuously discover and scan the external attack surface of one or more domains: " +
			"the domains themselves, their subdomains, and what they expose. " +
			"TLS, HTTP, DNS, email security and known-vulnerability checks always run; the attributes below turn the other scans on or off.",
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo space identifier. If there is no space ID, the provider space is used.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mrn": schema.StringAttribute{
				MarkdownDescription: "Integration identifier.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the integration. Defaults to the first domain.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
			"domains": schema.ListAttribute{
				MarkdownDescription: "Domains to scan, for example `example.com`. Lowercase, 1 to 100 of them.",
				Required:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					listvalidator.SizeBetween(1, 100),
					listvalidator.UniqueValues(),
					listvalidator.ValueStringsAre(stringvalidator.RegexMatches(
						regexp.MustCompile(`^([a-z0-9-]+\.)+[a-z]{2,}$`),
						"must be a lowercase domain name, like example.com",
					)),
				},
			},
			"schedule": schema.StringAttribute{
				MarkdownDescription: "How often to scan: a Go duration of at least `24h` (for example `24h`, `168h` or `336h`), " +
					"a cron expression, or `once`. With `once`, the provider starts that scan when it creates the integration, unless `pause_scanning` is `true`. Defaults to `24h`.",
				Optional: true,
				Computed: true,
				Default:  stringdefault.StaticString("24h"),
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"subdomains":             optionalBool("Discover and scan subdomains of the domains.", true),
			"port_scan":              optionalBool("Scan for open ports and exposed services.", true),
			"web_fingerprint":        optionalBool("Fingerprint web technologies.", true),
			"vulnerability_scanning": optionalBool("Run active vulnerability checks against discovered assets.", true),
			"subdomain_takeover":     optionalBool("Detect dangling DNS records vulnerable to subdomain takeover.", false),
			"cloud_discovery":        optionalBool("Discover cloud assets (AWS, Azure, Google Cloud) tied to the domains.", false),
			"webscan":                optionalBool("Run a dynamic web application scan. Slow, and can be intrusive.", false),
			"pause_scanning":         optionalBool("Pause scheduled scans.", false),
		},
	}
}

func (r *integrationAttackSurfaceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*ExtendedGqlClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ExtendedGqlClient. Got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	r.client = client
}

// configInput sends every setting: the API replaces the whole configuration on update
// and treats an unset toggle as off.
func (m integrationAttackSurfaceResourceModel) configInput() mondoov1.ClientIntegrationConfigurationInput {
	domains := ConvertSliceStrings(m.Domains)
	b := func(v types.Bool) *mondoov1.Boolean { return mondoov1.NewBooleanPtr(mondoov1.Boolean(v.ValueBool())) }
	return mondoov1.ClientIntegrationConfigurationInput{
		AttackSurfaceConfigurationOptions: &mondoov1.AttackSurfaceConfigurationOptionsInput{
			Domains:               &domains,
			Schedule:              mondoov1.NewStringPtr(mondoov1.String(m.Schedule.ValueString())),
			Subdomains:            b(m.Subdomains),
			PortScan:              b(m.PortScan),
			WebFingerprint:        b(m.WebFingerprint),
			VulnerabilityScanning: b(m.VulnerabilityScanning),
			SubdomainTakeover:     b(m.SubdomainTakeover),
			CloudDiscovery:        b(m.CloudDiscovery),
			Webscan:               b(m.Webscan),
			PauseScanning:         b(m.PauseScanning),
		},
	}
}

func attackSurfaceModelFrom(integration Integration) integrationAttackSurfaceResourceModel {
	o := integration.ConfigurationOptions.AttackSurfaceConfigurationOptions
	return integrationAttackSurfaceResourceModel{
		SpaceID:               types.StringValue(integration.SpaceID()),
		Mrn:                   types.StringValue(integration.Mrn),
		Name:                  types.StringValue(integration.Name),
		Domains:               ConvertListValue(o.Domains),
		Schedule:              types.StringValue(o.Schedule),
		Subdomains:            types.BoolValue(o.Subdomains),
		PortScan:              types.BoolValue(o.PortScan),
		WebFingerprint:        types.BoolValue(o.WebFingerprint),
		VulnerabilityScanning: types.BoolValue(o.VulnerabilityScanning),
		SubdomainTakeover:     types.BoolValue(o.SubdomainTakeover),
		CloudDiscovery:        types.BoolValue(o.CloudDiscovery),
		Webscan:               types.BoolValue(o.Webscan),
		PauseScanning:         types.BoolValue(o.PauseScanning),
	}
}

func (r *integrationAttackSurfaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationAttackSurfaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "space_mrn", space.MRN())

	if data.Name.IsNull() || data.Name.IsUnknown() {
		var domains []string
		resp.Diagnostics.Append(data.Domains.ElementsAs(ctx, &domains, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		data.Name = types.StringValue(domains[0])
	}

	tflog.Debug(ctx, "Creating attack surface integration")
	integration, err := r.client.CreateIntegration(ctx,
		space.MRN(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAttackSurface,
		data.configInput(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to create attack surface integration. Got error: %s", err))
		return
	}

	data.Mrn = types.StringValue(string(integration.Mrn))
	data.SpaceID = types.StringValue(space.ID())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

	// A recurring schedule starts scanning on its own; a one-time scan needs a start.
	if data.Schedule.ValueString() == "once" && !data.PauseScanning.ValueBool() {
		if _, err := r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunScan); err != nil {
			resp.Diagnostics.AddWarning("Client Error",
				fmt.Sprintf("Created the integration, but unable to start its scan. Got error: %s", err))
		}
	}
}

func (r *integrationAttackSurfaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationAttackSurfaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	integration, err := r.client.GetClientIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to read attack surface integration. Got error: %s", err))
		return
	}

	model := attackSurfaceModelFrom(integration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *integrationAttackSurfaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationAttackSurfaceResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAttackSurface,
		data.configInput(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to update attack surface integration. Got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAttackSurfaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationAttackSurfaceResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to delete attack surface integration. Got error: %s", err))
	}
}

func (r *integrationAttackSurfaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}
	model := attackSurfaceModelFrom(*integration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
