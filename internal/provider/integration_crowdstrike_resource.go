// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/setvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/setplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = (*integrationCrowdstrikeResource)(nil)
var _ resource.ResourceWithImportState = (*integrationCrowdstrikeResource)(nil)

func NewIntegrationCrowdstrikeResource() resource.Resource {
	return &integrationCrowdstrikeResource{}
}

type integrationCrowdstrikeResource struct {
	client *ExtendedGqlClient
}

type integrationCrowdstrikeResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn          types.String `tfsdk:"mrn"`
	Name         types.String `tfsdk:"name"`
	ClientId     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Cloud        types.String `tfsdk:"cloud"`
	MemberCID    types.String `tfsdk:"member_cid"`
	FindingTypes types.Set    `tfsdk:"finding_types"`
	Severities   types.Set    `tfsdk:"severities"`
}

// crowdstrikeFindingTypes are the values the API accepts for finding_types.
var crowdstrikeFindingTypes = []string{
	string(mondoov1.CrowdstrikeFalconFindingTypeVulnerability),
	string(mondoov1.CrowdstrikeFalconFindingTypeThreat),
}

// crowdstrikeSeverities are the values the API accepts for severities.
var crowdstrikeSeverities = []string{
	string(mondoov1.CrowdstrikeFalconSeverityCritical),
	string(mondoov1.CrowdstrikeFalconSeverityHigh),
	string(mondoov1.CrowdstrikeFalconSeverityMedium),
	string(mondoov1.CrowdstrikeFalconSeverityLow),
	string(mondoov1.CrowdstrikeFalconSeverityInformational),
}

// GetConfigurationOptions builds the API input. finding_types and severities
// are sent only when they hold a known value: the API keeps what is stored
// when either is omitted, and an explicit empty set clears it.
func (m integrationCrowdstrikeResourceModel) GetConfigurationOptions(ctx context.Context) (*mondoov1.CrowdstrikeFalconConfigurationOptionsInput, diag.Diagnostics) {
	var diags diag.Diagnostics
	opts := &mondoov1.CrowdstrikeFalconConfigurationOptionsInput{
		ClientId:     mondoov1.String(m.ClientId.ValueString()),
		ClientSecret: mondoov1.NewStringPtr(mondoov1.String(m.ClientSecret.ValueString())),
		Cloud:        mondoov1.NewStringPtr(mondoov1.String(m.Cloud.ValueString())),
		MemberCID:    mondoov1.NewStringPtr(mondoov1.String(m.MemberCID.ValueString())),
	}
	findingTypes, d := enumSetToInput[mondoov1.CrowdstrikeFalconFindingType](ctx, m.FindingTypes)
	diags.Append(d...)
	opts.FindingTypes = findingTypes
	severities, d := enumSetToInput[mondoov1.CrowdstrikeFalconSeverity](ctx, m.Severities)
	diags.Append(d...)
	opts.Severities = severities
	return opts, diags
}

// enumSetToInput converts a set of enum strings to an API list. A null or
// unknown set returns nil, so the field is left out of the request; a known
// empty set returns an empty list, which the API reads as "clear".
func enumSetToInput[T ~string](ctx context.Context, set types.Set) (*[]T, diag.Diagnostics) {
	if set.IsNull() || set.IsUnknown() {
		return nil, nil
	}
	var values []string
	diags := set.ElementsAs(ctx, &values, false)
	if diags.HasError() {
		return nil, diags
	}
	out := make([]T, 0, len(values))
	for _, v := range values {
		out = append(out, T(v))
	}
	return &out, diags
}

// enumSetValue converts an API list to a set. A nil list (nothing stored) is
// an empty set, not null, so it matches what the API reports as the default.
func enumSetValue(ctx context.Context, values []string) (types.Set, diag.Diagnostics) {
	if values == nil {
		values = []string{}
	}
	return types.SetValueFrom(ctx, types.StringType, values)
}

func (r *integrationCrowdstrikeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_crowdstrike"
}

func (r *integrationCrowdstrikeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "CrowdStrike Falcon for Cloud integration.",
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
				MarkdownDescription: "Integration identifier.",
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
				MarkdownDescription: "Client ID used for authentication with the CrowdStrike Falcon platform.",
				Required:            true,
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "Client secret used for authentication with the CrowdStrike Falcon platform.",
				Required:            true,
				Sensitive:           true,
			},
			"cloud": schema.StringAttribute{
				MarkdownDescription: "The Falcon Cloud to connect to.",
				Optional:            true,
			},
			"member_cid": schema.StringAttribute{
				MarkdownDescription: "CID selector for cases when the client ID and secret have access to multiple CIDs.",
				Optional:            true,
			},
			"finding_types": schema.SetAttribute{
				MarkdownDescription: "Kinds of findings to import besides Spotlight vulnerabilities, which are always imported. " +
					"Add `THREAT` to import Falcon endpoint alerts and quarantined files; the API client then needs the " +
					"`Alerts: Read` and `Quarantined Files: Read` scopes. Allowed values: `VULNERABILITY`, `THREAT`. " +
					"Omit it to keep what is stored (vulnerabilities only on a new integration); set it to `[]` to go back to vulnerabilities only.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(stringvalidator.OneOf(crowdstrikeFindingTypes...)),
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
			"severities": schema.SetAttribute{
				MarkdownDescription: "Import only findings of these severities. Allowed values: `CRITICAL`, `HIGH`, `MEDIUM`, `LOW`, `INFORMATIONAL`. " +
					"Empty imports every severity, and a finding whose severity CrowdStrike does not state is always imported. " +
					"Narrowing it closes the Spotlight vulnerabilities it now excludes at the next import; alerts and quarantined " +
					"files it excludes keep their last imported state. Omit it to keep what is stored (every severity on a new " +
					"integration); set it to `[]` to import every severity again.",
				ElementType: types.StringType,
				Optional:    true,
				Computed:    true,
				Validators: []validator.Set{
					setvalidator.ValueStringsAre(stringvalidator.OneOf(crowdstrikeSeverities...)),
				},
				PlanModifiers: []planmodifier.Set{
					setplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *integrationCrowdstrikeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationCrowdstrikeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationCrowdstrikeResourceModel

	// Read Terraform plan data into the model
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

	configOpts, diags := data.GetConfigurationOptions(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource.
	tflog.Debug(ctx, "Creating integration")
	integration, err := r.client.CreateIntegration(ctx,
		space.MRN(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeCrowdstrikeFalcon,
		mondoov1.ClientIntegrationConfigurationInput{
			CrowdstrikeFalconConfigurationOptions: configOpts,
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create %s integration. Got error: %s", mondoov1.IntegrationTypeCrowdstrikeFalcon, err),
			)
		return
	}

	// trigger integration to gather results quickly after the first setup
	// NOTE: we ignore the error since the integration state does not depend on it
	_, err = r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunImport)
	if err != nil {
		resp.Diagnostics.
			AddWarning("Client Error",
				fmt.Sprintf("Unable to trigger integration. Got error: %s", err),
			)
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))
	data.SpaceID = types.StringValue(space.ID())
	// Omitted on create, the API stores nothing: vulnerabilities only, every severity.
	if data.FindingTypes.IsUnknown() {
		data.FindingTypes, diags = enumSetValue(ctx, nil)
		resp.Diagnostics.Append(diags...)
	}
	if data.Severities.IsUnknown() {
		data.Severities, diags = enumSetValue(ctx, nil)
		resp.Diagnostics.Append(diags...)
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationCrowdstrikeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationCrowdstrikeResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationCrowdstrikeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationCrowdstrikeResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// finding_types and severities are Optional+Computed: when the config
	// leaves one out, the plan carries the prior state, but the API must not
	// be told to overwrite what it stores. Only send what the user configured.
	var cfgFindingTypes, cfgSeverities types.Set
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("finding_types"), &cfgFindingTypes)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("severities"), &cfgSeverities)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sendModel := data
	if cfgFindingTypes.IsNull() {
		sendModel.FindingTypes = types.SetNull(types.StringType)
	}
	if cfgSeverities.IsNull() {
		sendModel.Severities = types.SetNull(types.StringType)
	}
	configOpts, diags := sendModel.GetConfigurationOptions(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		CrowdstrikeFalconConfigurationOptions: configOpts,
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeCrowdstrikeFalcon,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update %s integration. Got error: %s", mondoov1.IntegrationTypeCrowdstrikeFalcon, err),
			)
		return
	}

	// A value is still unknown when the prior state predates the attribute and
	// the config leaves it out. It was not sent, so report what the API stores.
	if data.FindingTypes.IsUnknown() || data.Severities.IsUnknown() {
		r.fillStoredSelections(ctx, &data, &resp.Diagnostics)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationCrowdstrikeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationCrowdstrikeResourceModel

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
				fmt.Sprintf("Unable to delete %s integration. Got error: %s", mondoov1.IntegrationTypeCrowdstrikeFalcon, err),
			)
		return
	}
}

// fillStoredSelections sets unknown finding_types and severities from the
// integration the API stores. The update has already happened, so a failed
// read is a warning and falls back to the API's defaults (empty sets): the
// state must still be written.
func (r *integrationCrowdstrikeResource) fillStoredSelections(ctx context.Context, data *integrationCrowdstrikeResourceModel, diags *diag.Diagnostics) {
	var stored CrowdstrikeFalconConfigurationOptions
	integration, err := r.client.GetClientIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		diags.AddWarning("Client Error",
			fmt.Sprintf("Unable to read back finding_types and severities of %s integration; recording them as empty. Got error: %s",
				mondoov1.IntegrationTypeCrowdstrikeFalcon, err))
	} else {
		stored = integration.ConfigurationOptions.CrowdstrikeFalconConfigurationOptions
	}
	var d diag.Diagnostics
	if data.FindingTypes.IsUnknown() {
		data.FindingTypes, d = enumSetValue(ctx, stored.FindingTypes)
		diags.Append(d...)
	}
	if data.Severities.IsUnknown() {
		data.Severities, d = enumSetValue(ctx, stored.Severities)
		diags.Append(d...)
	}
}

func (r *integrationCrowdstrikeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	stored := integration.ConfigurationOptions.CrowdstrikeFalconConfigurationOptions
	findingTypes, diags := enumSetValue(ctx, stored.FindingTypes)
	resp.Diagnostics.Append(diags...)
	severities, diags := enumSetValue(ctx, stored.Severities)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	model := integrationCrowdstrikeResourceModel{
		Mrn:          types.StringValue(integration.Mrn),
		Name:         types.StringValue(integration.Name),
		SpaceID:      types.StringValue(integration.SpaceID()),
		ClientId:     types.StringValue(stored.ClientId),
		ClientSecret: types.StringPointerValue(nil),
		Cloud:        types.StringValue(stored.Cloud),
		MemberCID:    types.StringPointerValue(nil),
		FindingTypes: findingTypes,
		Severities:   severities,
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
