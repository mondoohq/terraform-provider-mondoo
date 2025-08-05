// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/objectplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &SpaceResource{}
var _ resource.ResourceWithImportState = &SpaceResource{}

func NewSpaceResource() resource.Resource {
	return &SpaceResource{}
}

// SpaceResource defines the resource implementation.
type SpaceResource struct {
	client *ExtendedGqlClient
}

// SpaceModel describes the resource data model.
type SpaceModel struct {
	SpaceID       types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	Description   types.String `tfsdk:"description"`
	OrgID         types.String `tfsdk:"org_id"`
	SpaceMrn      types.String `tfsdk:"mrn"`
	SpaceSettings types.Object `tfsdk:"space_settings"`
}

type SpaceSettingsInput struct {
	TerminatedAssetsConfiguration      *TerminatedAssetsConfiguration      `tfsdk:"terminated_assets_configuration"`
	UnusedServiceAccountsConfiguration *UnusedServiceAccountsConfiguration `tfsdk:"unused_service_accounts_configuration"`
	GarbageCollectAssetsConfiguration  *GarbageCollectAssetsConfiguration  `tfsdk:"garbage_collect_assets_configuration"`
	PlatformVulnerabilityConfiguration *PlatformVulnerabilityConfiguration `tfsdk:"platform_vulnerability_configuration"`
	EolAssetsConfiguration             *EolAssetsConfiguration             `tfsdk:"eol_assets_configuration"`
	CasesConfiguration                 *CasesConfiguration                 `tfsdk:"cases_configuration"`
	ExceptionsConfiguration            *ExceptionsConfiguration            `tfsdk:"exceptions_configuration"`
}

type TerminatedAssetsConfiguration struct {
	Cleanup types.Bool `tfsdk:"cleanup"`
}

type UnusedServiceAccountsConfiguration struct {
	Cleanup types.Bool `tfsdk:"cleanup"`
}

type GarbageCollectAssetsConfiguration struct {
	Enabled   types.Bool  `tfsdk:"enabled"`
	AfterDays types.Int32 `tfsdk:"after_days"`
}

type PlatformVulnerabilityConfiguration struct {
	Enabled types.Bool `tfsdk:"enabled"`
}

type EolAssetsConfiguration struct {
	Enabled         types.Bool  `tfsdk:"enabled"`
	MonthsInAdvance types.Int32 `tfsdk:"months_in_advance"`
}

type CasesConfiguration struct {
	AutoCreate        types.Bool  `tfsdk:"auto_create"`
	AggregationWindow types.Int32 `tfsdk:"aggregation_window"`
}

type ExceptionsConfiguration struct {
	RequireApproval           types.Bool `tfsdk:"require_approval"`
	AllowIndefiniteValidUntil types.Bool `tfsdk:"allow_indefinite_valid_until"`
	AllowSelfApproval         types.Bool `tfsdk:"allow_self_approval"`
}

func (r *SpaceResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_space"
}

func SpaceSettingsInputAttrTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"terminated_assets_configuration":       types.ObjectType{AttrTypes: map[string]attr.Type{"cleanup": types.BoolType}},
		"unused_service_accounts_configuration": types.ObjectType{AttrTypes: map[string]attr.Type{"cleanup": types.BoolType}},
		"garbage_collect_assets_configuration":  types.ObjectType{AttrTypes: map[string]attr.Type{"enabled": types.BoolType, "after_days": types.Int32Type}},
		"platform_vulnerability_configuration":  types.ObjectType{AttrTypes: map[string]attr.Type{"enabled": types.BoolType}},
		"eol_assets_configuration":              types.ObjectType{AttrTypes: map[string]attr.Type{"enabled": types.BoolType, "months_in_advance": types.Int32Type}},
		"cases_configuration":                   types.ObjectType{AttrTypes: map[string]attr.Type{"auto_create": types.BoolType, "aggregation_window": types.Int32Type}},
		"exceptions_configuration": types.ObjectType{AttrTypes: map[string]attr.Type{
			"require_approval":             types.BoolType,
			"allow_indefinite_valid_until": types.BoolType,
			"allow_self_approval":          types.BoolType,
		}},
	}
}

func SpaceSettingsInputToObject(ctx context.Context, input *SpaceSettingsInput) (types.Object, diag.Diagnostics) {
	if input == nil {
		return types.ObjectNull(SpaceSettingsInputAttrTypes()), nil
	}
	return types.ObjectValueFrom(ctx, SpaceSettingsInputAttrTypes(), input)
}

func ObjectToSpaceSettingsInput(ctx context.Context, obj types.Object) (*SpaceSettingsInput, diag.Diagnostics) {
	if obj.IsNull() || obj.IsUnknown() {
		return nil, nil
	}
	var result SpaceSettingsInput
	diags := obj.As(ctx, &result, basetypes.ObjectAsOptions{})
	if diags.HasError() {
		return nil, diags
	}
	return &result, nil
}

func (r *SpaceResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Space resource",

		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the space.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^([a-zA-Z \-'_]|\d){2,30}$`),
						"must contain 2 to 30 characters, where each character can be a letter (uppercase or lowercase), a space, a dash, an underscore, or a digit",
					),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the space.",
				Optional:            true,
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "ID of the space. Must be globally unique. If the provider has a space configured and this field is empty, the provider space is used.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-z\d]([\d-_]|[a-z]){2,48}[a-z\d]$`),
						"must contain 4 to 50 digits, dashes, underscores, or lowercase letters, and ending with either a lowercase letter or a digit",
					),
				},
			},
			"mrn": schema.StringAttribute{
				MarkdownDescription: "Mrn of the space.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"org_id": schema.StringAttribute{
				MarkdownDescription: "ID of the organization.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[a-z\d]([\d-_]|[a-z]){4,48}[a-z\d]$`),
						"must contain 6 to 50 digits, dashes, underscores, or lowercase letters, and ending with either a lowercase letter or a digit",
					),
				},
			},
			"space_settings": schema.SingleNestedAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Space settings.",
				PlanModifiers: []planmodifier.Object{
					objectplanmodifier.UseStateForUnknown(),
				},
				Attributes: map[string]schema.Attribute{
					"terminated_assets_configuration": schema.SingleNestedAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Terminated assets configuration for the space.",
						Attributes: map[string]schema.Attribute{
							"cleanup": schema.BoolAttribute{
								Required:            true,
								MarkdownDescription: "Whether to cleanup terminated assets.",
							},
						},
					},
					"unused_service_accounts_configuration": schema.SingleNestedAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Unused service accounts configuration for the space.",
						Attributes: map[string]schema.Attribute{
							"cleanup": schema.BoolAttribute{
								Required:            true,
								MarkdownDescription: "Whether to cleanup unused service accounts.",
							},
						},
					},
					"garbage_collect_assets_configuration": schema.SingleNestedAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Garbage collect assets configuration for the space.",
						PlanModifiers: []planmodifier.Object{
							objectplanmodifier.UseStateForUnknown(),
						},
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Whether to enable garbage collection.",
							},
							"after_days": schema.Int32Attribute{
								Optional:            true,
								MarkdownDescription: "After how many days to garbage collect. ",
							},
						},
					},
					"platform_vulnerability_configuration": schema.SingleNestedAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Platform vulnerability configuration for the space.",
						PlanModifiers: []planmodifier.Object{
							objectplanmodifier.UseStateForUnknown(),
						},
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Required:            true,
								MarkdownDescription: "Whether to enable platform vulnerability analysis.",
							},
						},
					},
					"eol_assets_configuration": schema.SingleNestedAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "EOL platform configuration for the space.",
						PlanModifiers: []planmodifier.Object{
							objectplanmodifier.UseStateForUnknown(),
						},
						Attributes: map[string]schema.Attribute{
							"enabled": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Whether to enable EOL assets analysis.",
							},
							"months_in_advance": schema.Int32Attribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "How many months in advance should EOL be applied as risk factor.",
							},
						},
					},
					"cases_configuration": schema.SingleNestedAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Cases configuration for the space.",
						PlanModifiers: []planmodifier.Object{
							objectplanmodifier.UseStateForUnknown(),
						},
						Attributes: map[string]schema.Attribute{
							"auto_create": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Whether to enable auto-create cases on drift.",
							},
							"aggregation_window": schema.Int32Attribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Aggregate findings for the same asset within this window. The value is specified in hours. 0 means no aggregation.",
							},
						},
					},
					"exceptions_configuration": schema.SingleNestedAttribute{
						Optional:            true,
						Computed:            true,
						MarkdownDescription: "Exceptions configuration for the space.",
						PlanModifiers: []planmodifier.Object{
							objectplanmodifier.UseStateForUnknown(),
						},
						Attributes: map[string]schema.Attribute{
							"require_approval": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Whether to require approval for exceptions.",
							},
							"allow_indefinite_valid_until": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Whether to allow creation of exception groups with indefinite valid until.",
							},
							"allow_self_approval": schema.BoolAttribute{
								Optional:            true,
								Computed:            true,
								MarkdownDescription: "Whether a user can approve their own exception requests.",
							},
						},
					},
				},
			},
		},
	}
}

func (r *SpaceResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *mondoov1.Client. Got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *SpaceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SpaceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Compute and validate the space
	var spaceID *mondoov1.String
	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		// we do not fail if the user doesn't specify an id
		// because we are creating one, still log the error
		tflog.Debug(ctx, err.Error())
	} else {
		spaceID = mondoov1.NewStringPtr(mondoov1.String(space.ID()))
	}

	spaceSettings, diags := ObjectToSpaceSettingsInput(ctx, data.SpaceSettings)
	if diags.HasError() {
		resp.Diagnostics = append(resp.Diagnostics, diags...)
		return
	}

	createInput := mondoov1.CreateSpaceInput{
		Name:        mondoov1.String(data.Name.ValueString()),
		Description: mondoov1.NewStringPtr(mondoov1.String(data.Description.ValueString())),
		Id:          spaceID,
		OrgMrn:      mondoov1.String(orgPrefix + data.OrgID.ValueString()),
		Settings:    ExpandSpaceSettings(spaceSettings),
	}

	// Do GraphQL request to API to create the resource.
	payload, err := r.client.CreateSpace(ctx, createInput)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create space. Got error: %s", err),
			)
		return
	}

	// Save space mrn into the Terraform state.
	data.Name = types.StringValue(string(payload.Name))

	id, ok := payload.Id.(string)
	if ok {
		data.SpaceID = types.StringValue(id)
		ctx = tflog.SetField(ctx, "space_id", data.SpaceID)
	}

	data.SpaceMrn = types.StringValue(string(payload.Mrn))
	ctx = tflog.SetField(ctx, "space_mrn", data.SpaceMrn)

	// Write logs using the tflog package
	tflog.Debug(ctx, "Created a space resource")

	data.SpaceSettings, diags = SpaceSettingsInputToObject(ctx, FlattenSpaceSettingsInput(payload.Settings))
	if diags.HasError() {
		resp.Diagnostics = append(resp.Diagnostics, diags...)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SpaceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SpaceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// nothing to do here, we already have the data in the state

	spacePayload, err := r.client.GetSpace(ctx, data.SpaceMrn.ValueString())
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to retrieve space. Got error: %s", err),
			)
		return
	}

	spaceSettings, diags := SpaceSettingsInputToObject(ctx, FlattenSpaceSettingsInput(spacePayload.Settings))
	if diags.HasError() {
		resp.Diagnostics = append(resp.Diagnostics, diags...)
		return
	}

	model := SpaceModel{
		SpaceID:       types.StringValue(spacePayload.Id),
		SpaceMrn:      types.StringValue(spacePayload.Mrn),
		Name:          types.StringValue(spacePayload.Name),
		OrgID:         types.StringValue(spacePayload.Organization.Id),
		SpaceSettings: spaceSettings,
	}

	if spacePayload.Description != "" {
		model.Description = types.StringValue(spacePayload.Description)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *SpaceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SpaceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Compute and validate the space
	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		// we do not fail if there the user doesn't specify an id
		// because we are creating one, still log the error
		tflog.Debug(ctx, err.Error())
	}
	ctx = tflog.SetField(ctx, "computed_space_id", space.ID())

	// ensure space id is not changed
	var planSpaceID string
	req.Plan.GetAttribute(ctx, path.Root("id"), &planSpaceID)

	if space.ID() != planSpaceID {
		resp.Diagnostics.AddError(
			"Space ID cannot be changed",
			"Note that the Mondoo space can be configured at the resource or provider level.",
		)
		return
	}
	ctx = tflog.SetField(ctx, "space_id_from_plan", planSpaceID)

	spaceSettings, diags := ObjectToSpaceSettingsInput(ctx, data.SpaceSettings)
	if diags.HasError() {
		resp.Diagnostics = append(resp.Diagnostics, diags...)
		return
	}

	// Do GraphQL request to API to update the resource.
	tflog.Debug(ctx, "Updating space")
	err = r.client.UpdateSpace(ctx,
		planSpaceID,
		data.Name.ValueString(),
		data.Description.ValueString(),
		ExpandSpaceSettings(spaceSettings),
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error", fmt.Sprintf("Unable to update space. Got error: %s", err))
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SpaceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SpaceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to delete the resource.
	err := r.client.DeleteSpace(ctx, data.SpaceID.ValueString())
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to delete space. Got error: %s", err),
			)
		return
	}
}

func (r *SpaceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	mrn := spacePrefix + req.ID
	spacePayload, err := r.client.GetSpace(ctx, mrn)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to retrieve space. Got error: %s", err),
			)
		return
	}

	spaceSettings, diags := SpaceSettingsInputToObject(ctx, FlattenSpaceSettingsInput(spacePayload.Settings))
	if diags.HasError() {
		resp.Diagnostics = append(resp.Diagnostics, diags...)
		return
	}

	model := SpaceModel{
		SpaceID:       types.StringValue(spacePayload.Id),
		SpaceMrn:      types.StringValue(spacePayload.Mrn),
		Name:          types.StringValue(spacePayload.Name),
		OrgID:         types.StringValue(spacePayload.Organization.Id),
		SpaceSettings: spaceSettings,
	}

	if spacePayload.Description != "" {
		model.Description = types.StringValue(spacePayload.Description)
	}

	resp.State.Set(ctx, &model)
}

func ExpandSpaceSettings(settings *SpaceSettingsInput) *mondoov1.SpaceSettingsInput {
	if settings == nil {
		return nil
	}

	return &mondoov1.SpaceSettingsInput{
		TerminatedAssetsConfiguration:      expandTerminatedAssetsConfig(settings.TerminatedAssetsConfiguration),
		UnusedServiceAccountsConfiguration: expandUnusedServiceAccountsConfig(settings.UnusedServiceAccountsConfiguration),
		GarbageCollectAssetsConfiguration:  expandGarbageCollectAssetsConfig(settings.GarbageCollectAssetsConfiguration),
		PlatformVulnerabilityConfiguration: expandPlatformVulnConfig(settings.PlatformVulnerabilityConfiguration),
		EolAssetsConfiguration:             expandEolAssetsConfig(settings.EolAssetsConfiguration),
		CasesConfiguration:                 expandCasesConfig(settings.CasesConfiguration),
		ExceptionsConfiguration:            expandExceptionsConfig(settings.ExceptionsConfiguration),
	}
}

func expandTerminatedAssetsConfig(cfg *TerminatedAssetsConfiguration) *mondoov1.TerminatedAssetsConfigurationInput {
	if cfg == nil || cfg.Cleanup.IsNull() {
		return nil
	}
	return &mondoov1.TerminatedAssetsConfigurationInput{
		Cleanup: mondoov1.NewBooleanPtr(mondoov1.Boolean(cfg.Cleanup.ValueBool())),
	}
}

func expandUnusedServiceAccountsConfig(cfg *UnusedServiceAccountsConfiguration) *mondoov1.UnusedServiceAccountsConfigurationInput {
	if cfg == nil || cfg.Cleanup.IsNull() {
		return nil
	}
	return &mondoov1.UnusedServiceAccountsConfigurationInput{
		Cleanup: mondoov1.NewBooleanPtr(mondoov1.Boolean(cfg.Cleanup.ValueBool())),
	}
}

func expandGarbageCollectAssetsConfig(cfg *GarbageCollectAssetsConfiguration) *mondoov1.GarbageCollectAssetsConfigurationInput {
	if cfg == nil {
		return nil
	}

	input := &mondoov1.GarbageCollectAssetsConfigurationInput{}
	empty := true

	if !cfg.Enabled.IsNull() {
		input.Enable = mondoov1.NewBooleanPtr(mondoov1.Boolean(cfg.Enabled.ValueBool()))
		empty = false
	}
	if !cfg.AfterDays.IsNull() {
		input.AfterDays = mondoov1.NewIntPtr(mondoov1.Int(cfg.AfterDays.ValueInt32()))
		empty = false
	}

	if empty {
		return nil
	}
	return input
}

func expandPlatformVulnConfig(cfg *PlatformVulnerabilityConfiguration) *mondoov1.PlatformVulnerabilityConfigurationInput {
	if cfg == nil || cfg.Enabled.IsNull() {
		return nil
	}
	return &mondoov1.PlatformVulnerabilityConfigurationInput{
		Enable: mondoov1.NewBooleanPtr(mondoov1.Boolean(cfg.Enabled.ValueBool())),
	}
}

func expandEolAssetsConfig(cfg *EolAssetsConfiguration) *mondoov1.EolAssetsConfigurationInput {
	if cfg == nil {
		return nil
	}

	input := &mondoov1.EolAssetsConfigurationInput{}
	empty := true

	if !cfg.Enabled.IsNull() {
		input.Enable = mondoov1.NewBooleanPtr(mondoov1.Boolean(cfg.Enabled.ValueBool()))
		empty = false
	}
	if !cfg.MonthsInAdvance.IsNull() {
		input.MonthsInAdvance = mondoov1.NewIntPtr(mondoov1.Int(cfg.MonthsInAdvance.ValueInt32()))
		empty = false
	}

	if empty {
		return nil
	}
	return input
}

func expandCasesConfig(cfg *CasesConfiguration) *mondoov1.CasesConfigurationInput {
	if cfg == nil {
		return nil
	}

	input := &mondoov1.CasesConfigurationInput{}
	empty := true

	if !cfg.AutoCreate.IsNull() {
		input.AutoCreate = mondoov1.NewBooleanPtr(mondoov1.Boolean(cfg.AutoCreate.ValueBool()))
		empty = false
	}
	if !cfg.AggregationWindow.IsNull() {
		input.AggregationWindow = mondoov1.NewIntPtr(mondoov1.Int(cfg.AggregationWindow.ValueInt32()))
		empty = false
	}

	if empty {
		return nil
	}
	return input
}

func expandExceptionsConfig(cfg *ExceptionsConfiguration) *mondoov1.ExceptionsConfigurationInput {
	if cfg == nil {
		return nil
	}

	input := &mondoov1.ExceptionsConfigurationInput{}
	empty := true

	if !cfg.RequireApproval.IsNull() {
		input.RequireApproval = mondoov1.NewBooleanPtr(mondoov1.Boolean(cfg.RequireApproval.ValueBool()))
		empty = false
	}
	if !cfg.AllowIndefiniteValidUntil.IsNull() {
		input.AllowIndefiniteValidUntil = mondoov1.NewBooleanPtr(mondoov1.Boolean(cfg.AllowIndefiniteValidUntil.ValueBool()))
		empty = false
	}
	if !cfg.AllowSelfApproval.IsNull() {
		input.AllowSelfApproval = mondoov1.NewBooleanPtr(mondoov1.Boolean(cfg.AllowSelfApproval.ValueBool()))
		empty = false
	}

	if empty {
		return nil
	}
	return input
}

func FlattenSpaceSettingsInput(input *MondooSpaceSettingsInput) *SpaceSettingsInput {
	if input == nil {
		return &SpaceSettingsInput{}
	}

	return &SpaceSettingsInput{
		TerminatedAssetsConfiguration:      flattenTerminatedAssetsConfig(input.TerminatedAssetsConfiguration),
		UnusedServiceAccountsConfiguration: flattenUnusedServiceAccountsConfig(input.UnusedServiceAccountsConfiguration),
		GarbageCollectAssetsConfiguration:  flattenGarbageCollectAssetsConfig(input.GarbageCollectAssetsConfiguration),
		PlatformVulnerabilityConfiguration: flattenPlatformVulnConfig(input.PlatformVulnerabilityConfiguration),
		EolAssetsConfiguration:             flattenEolAssetsConfig(input.EolAssetsConfiguration),
		CasesConfiguration:                 flattenCasesConfig(input.CasesConfiguration),
		ExceptionsConfiguration:            flattenExceptionsConfig(input.ExceptionsConfiguration),
	}
}

func flattenTerminatedAssetsConfig(in *mondoov1.TerminatedAssetsConfigurationInput) *TerminatedAssetsConfiguration {
	if in == nil || in.Cleanup == nil {
		return nil
	}
	return &TerminatedAssetsConfiguration{
		Cleanup: types.BoolValue(bool(*in.Cleanup)),
	}
}

func flattenUnusedServiceAccountsConfig(in *mondoov1.UnusedServiceAccountsConfigurationInput) *UnusedServiceAccountsConfiguration {
	if in == nil || in.Cleanup == nil {
		return nil
	}
	return &UnusedServiceAccountsConfiguration{
		Cleanup: types.BoolValue(bool(*in.Cleanup)),
	}
}

func flattenGarbageCollectAssetsConfig(in *mondoov1.GarbageCollectAssetsConfigurationInput) *GarbageCollectAssetsConfiguration {
	if in == nil {
		return nil
	}
	out := &GarbageCollectAssetsConfiguration{}
	if in.Enable != nil {
		out.Enabled = types.BoolValue(bool(*in.Enable))
	} else {
		out.Enabled = types.BoolNull()
	}
	if in.AfterDays != nil {
		out.AfterDays = types.Int32Value(int32(*in.AfterDays))
	} else {
		out.AfterDays = types.Int32Null()
	}
	return out
}

func flattenPlatformVulnConfig(in *mondoov1.PlatformVulnerabilityConfigurationInput) *PlatformVulnerabilityConfiguration {
	if in == nil || in.Enable == nil {
		return nil
	}
	return &PlatformVulnerabilityConfiguration{
		Enabled: types.BoolValue(bool(*in.Enable)),
	}
}

func flattenEolAssetsConfig(in *mondoov1.EolAssetsConfigurationInput) *EolAssetsConfiguration {
	if in == nil {
		return nil
	}
	out := &EolAssetsConfiguration{}
	if in.Enable != nil {
		out.Enabled = types.BoolValue(bool(*in.Enable))
	} else {
		out.Enabled = types.BoolNull()
	}
	if in.MonthsInAdvance != nil {
		out.MonthsInAdvance = types.Int32Value(int32(*in.MonthsInAdvance))
	} else {
		out.MonthsInAdvance = types.Int32Null()
	}
	return out
}

func flattenCasesConfig(in *mondoov1.CasesConfigurationInput) *CasesConfiguration {
	if in == nil {
		return nil
	}
	out := &CasesConfiguration{}
	if in.AutoCreate != nil {
		out.AutoCreate = types.BoolValue(bool(*in.AutoCreate))
	} else {
		out.AutoCreate = types.BoolNull()
	}
	if in.AggregationWindow != nil {
		out.AggregationWindow = types.Int32Value(int32(*in.AggregationWindow))
	} else {
		out.AggregationWindow = types.Int32Null()
	}
	return out
}

func flattenExceptionsConfig(in *mondoov1.ExceptionsConfigurationInput) *ExceptionsConfiguration {
	if in == nil {
		return nil
	}
	out := &ExceptionsConfiguration{}
	if in.RequireApproval != nil {
		out.RequireApproval = types.BoolValue(bool(*in.RequireApproval))
	} else {
		out.RequireApproval = types.BoolNull()
	}
	if in.AllowIndefiniteValidUntil != nil {
		out.AllowIndefiniteValidUntil = types.BoolValue(bool(*in.AllowIndefiniteValidUntil))
	} else {
		out.AllowIndefiniteValidUntil = types.BoolNull()
	}
	if in.AllowSelfApproval != nil {
		out.AllowSelfApproval = types.BoolValue(bool(*in.AllowSelfApproval))
	} else {
		out.AllowSelfApproval = types.BoolNull()
	}
	return out
}
