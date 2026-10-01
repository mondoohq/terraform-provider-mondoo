// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var (
	_ resource.Resource                   = (*OrganizationSlaResource)(nil)
	_ resource.ResourceWithConfigure      = (*OrganizationSlaResource)(nil)
	_ resource.ResourceWithImportState    = (*OrganizationSlaResource)(nil)
	_ resource.ResourceWithValidateConfig = (*OrganizationSlaResource)(nil)
)

func NewOrganizationSlaResource() resource.Resource {
	return &OrganizationSlaResource{}
}

// OrganizationSlaResource manages the SLAs of a Mondoo organization. The
// organization's SLAs are a singleton: setting them governs every space in the
// organization, and clearing them hands each space back its own SLAs.
type OrganizationSlaResource struct {
	client *ExtendedGqlClient
}

type OrganizationSlaResourceModel struct {
	OrgID           types.String `tfsdk:"org_id"`
	OrgMrn          types.String `tfsdk:"org_mrn"`
	Critical        types.Object `tfsdk:"critical"`
	High            types.Object `tfsdk:"high"`
	Medium          types.Object `tfsdk:"medium"`
	Low             types.Object `tfsdk:"low"`
	StartDateConfig types.String `tfsdk:"start_date_config"`
	RatingSource    types.String `tfsdk:"rating_source"`
}

// organizationSlaRatingModel is the SLA for one finding rating.
type organizationSlaRatingModel struct {
	DaysToResolve     types.Int32 `tfsdk:"days_to_resolve"`
	DaysBeforeWarning types.Int32 `tfsdk:"days_before_warning"`
}

var organizationSlaRatingAttrTypes = map[string]attr.Type{
	"days_to_resolve":     types.Int32Type,
	"days_before_warning": types.Int32Type,
}

// organizationSlaRatings pairs each rating attribute with the API rating it
// configures, in the order the API lists them.
var organizationSlaRatings = []struct {
	attribute string
	rating    mondoov1.ScoreRating
}{
	{attribute: "critical", rating: mondoov1.ScoreRatingCritical},
	{attribute: "high", rating: mondoov1.ScoreRatingHigh},
	{attribute: "medium", rating: mondoov1.ScoreRatingMedium},
	{attribute: "low", rating: mondoov1.ScoreRatingLow},
}

// The API's defaults for the optional settings, also documented in the
// GraphQL schema.
const (
	organizationSlaDefaultStartDateConfig = string(mondoov1.SLAStartDateConfigCveDetected)
	organizationSlaDefaultRatingSource    = string(mondoov1.SLARatingSourceRisk)
)

// ratingObject returns the model field that holds the SLA for a rating
// attribute.
func (m *OrganizationSlaResourceModel) ratingObject(attribute string) *types.Object {
	switch attribute {
	case "critical":
		return &m.Critical
	case "high":
		return &m.High
	case "medium":
		return &m.Medium
	case "low":
		return &m.Low
	default:
		return nil
	}
}

func (r *OrganizationSlaResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_sla"
}

func organizationSlaRatingAttribute(rating string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: fmt.Sprintf("SLA for findings rated %s.", rating),
		Required:            true,
		Attributes: map[string]schema.Attribute{
			"days_to_resolve": schema.Int32Attribute{
				MarkdownDescription: "Number of days after the SLA start date (see `start_date_config`) by which a finding with this rating must be resolved. Must be at least `days_before_warning`.",
				Required:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
				},
			},
			"days_before_warning": schema.Int32Attribute{
				MarkdownDescription: "Number of days after the SLA start date at which a finding with this rating is flagged as approaching its deadline. Must not be greater than `days_to_resolve`.",
				Required:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
				},
			},
		},
	}
}

func (r *OrganizationSlaResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Sets the SLAs of a Mondoo organization: how many days a vulnerability finding of each rating has to be resolved, and when it is flagged as approaching its deadline.

This is an **authoritative** resource. It manages the organization's whole SLA configuration, sets every value on each apply, and there can be only one per organization.

While this resource exists, every space in the organization, including spaces created later, uses these SLAs, and nobody can change the SLAs in a space. Destroying this resource removes the organization's SLAs: every space can edit its SLAs again and goes back to its own SLA settings, or to the Mondoo defaults if it never set any. A space's own settings are kept, unchanged, while the organization's SLAs apply.

Changing the SLAs doesn't move existing deadlines right away: each finding's deadline follows the new SLAs on the asset's next scan.

Managing this resource requires permission to change the organization's SLAs, which the organization's Owner, Editor, and SLA Manager roles grant.

~> **Note:** If the organization's SLAs were already set in the Mondoo console, creating this resource overwrites them and destroying it removes them. To start from the current values, import them first.`,

		Attributes: map[string]schema.Attribute{
			"org_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the organization, not its MRN. Changing it moves the SLAs: they are removed from the old organization and set on the new one.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^[^/\s]+$`),
						"must be an organization ID, such as \"my-org-123456\", not an organization MRN",
					),
				},
			},
			"org_mrn": schema.StringAttribute{
				MarkdownDescription: "The Mondoo Resource Name (MRN) of the organization.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"critical": organizationSlaRatingAttribute("critical"),
			"high":     organizationSlaRatingAttribute("high"),
			"medium":   organizationSlaRatingAttribute("medium"),
			"low":      organizationSlaRatingAttribute("low"),
			"start_date_config": schema.StringAttribute{
				MarkdownDescription: "When a finding's SLA starts: `CVE_DETECTED` (when Mondoo first detects the vulnerability on the asset) or `CVE_PUBLISHED` (when the CVE was published). Defaults to `CVE_DETECTED`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(organizationSlaDefaultStartDateConfig),
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(mondoov1.SLAStartDateConfigCveDetected),
						string(mondoov1.SLAStartDateConfigCvePublished),
					),
				},
			},
			"rating_source": schema.StringAttribute{
				MarkdownDescription: "Which rating decides a finding's SLA: `RISK` (the Mondoo risk rating) or `CVSS` (the CVSS base score). Defaults to `RISK`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(organizationSlaDefaultRatingSource),
				Validators: []validator.String{
					stringvalidator.OneOf(
						string(mondoov1.SLARatingSourceRisk),
						string(mondoov1.SLARatingSourceCvss),
					),
				},
			},
		},
	}
}

func (r *OrganizationSlaResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ExtendedGqlClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}
	r.client = client
}

// ValidateConfig rejects a rating whose warning comes after its deadline, so
// the mistake surfaces at plan time instead of failing at apply.
func (r *OrganizationSlaResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data OrganizationSlaResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateOrganizationSlaRatings(ctx, &data)...)
}

// validateOrganizationSlaRatings checks days_to_resolve >= days_before_warning
// for each rating. Ratings or values not known yet (references to other
// resources) are skipped.
func validateOrganizationSlaRatings(ctx context.Context, data *OrganizationSlaResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics
	for _, r := range organizationSlaRatings {
		obj := data.ratingObject(r.attribute)
		if obj.IsNull() || obj.IsUnknown() {
			continue
		}

		var rating organizationSlaRatingModel
		d := obj.As(ctx, &rating, basetypes.ObjectAsOptions{})
		diags.Append(d...)
		if d.HasError() {
			continue
		}

		if !isKnownInt32(rating.DaysToResolve) || !isKnownInt32(rating.DaysBeforeWarning) {
			continue
		}
		if rating.DaysToResolve.ValueInt32() < rating.DaysBeforeWarning.ValueInt32() {
			diags.AddAttributeError(
				path.Root(r.attribute).AtName("days_before_warning"),
				"Invalid SLA",
				fmt.Sprintf(
					"%s: days_before_warning (%d) must not be greater than days_to_resolve (%d).",
					r.attribute, rating.DaysBeforeWarning.ValueInt32(), rating.DaysToResolve.ValueInt32(),
				),
			)
		}
	}
	return diags
}

func isKnownInt32(v types.Int32) bool {
	return !v.IsNull() && !v.IsUnknown()
}

func (r *OrganizationSlaResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationSlaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.setOrganizationSLAs(ctx, &data, &resp.Diagnostics, "create") {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSlaResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationSlaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgMrn := orgPrefix + data.OrgID.ValueString()
	model, err := r.client.GetSecurityModel(ctx, orgMrn)
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read organization SLAs", err.Error())
		return
	}

	// The organization no longer sets SLAs: they were cleared outside of
	// Terraform, so the next plan creates them again.
	if model == nil || model.SlasSource != slaConfigurationSourceOrganization {
		tflog.Warn(ctx, "organization no longer sets SLAs, removing from state", map[string]interface{}{
			"orgMrn": orgMrn,
		})
		resp.State.RemoveResource(ctx)
		return
	}

	data.OrgMrn = types.StringValue(orgMrn)
	resp.Diagnostics.Append(flattenOrganizationSLAs(model.Slas, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationSlaResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrganizationSlaResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.setOrganizationSLAs(ctx, &data, &resp.Diagnostics, "update") {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// setOrganizationSLAs writes the planned SLAs to the organization and fills
// in org_mrn. It reports whether the write succeeded. Every rating and both
// settings are always sent, so the API's merge replaces whatever was stored.
// The state then holds exactly what was sent, which is what the API stores.
func (r *OrganizationSlaResource) setOrganizationSLAs(ctx context.Context, data *OrganizationSlaResourceModel, diags *diag.Diagnostics, verb string) bool {
	orgMrn := orgPrefix + data.OrgID.ValueString()

	slas, d := expandOrganizationSLAs(ctx, data)
	diags.Append(d...)
	if diags.HasError() {
		return false
	}

	tflog.Debug(ctx, "setting organization SLAs", map[string]interface{}{
		"orgMrn": orgMrn,
	})

	err := r.client.UpdateSecurityModel(ctx, mondoov1.UpdateSecurityModelInput{
		ScopeMrn: mondoov1.String(orgMrn),
		Slas:     &slas,
	})
	if err != nil {
		diags.AddError(fmt.Sprintf("Failed to %s organization SLAs", verb), err.Error())
		return false
	}

	data.OrgMrn = types.StringValue(orgMrn)
	return true
}

func (r *OrganizationSlaResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrganizationSlaResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgMrn := orgPrefix + data.OrgID.ValueString()
	tflog.Debug(ctx, "clearing organization SLAs", map[string]interface{}{
		"orgMrn": orgMrn,
	})

	// Clearing an organization that sets no SLAs is a no-op on the API side,
	// so SLAs already cleared outside of Terraform don't fail the destroy.
	err := r.client.ClearSecurityModel(ctx, orgMrn)
	if err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to clear organization SLAs", err.Error())
	}
}

// ImportState accepts an organization ID or an organization MRN.
func (r *OrganizationSlaResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	orgID, err := organizationIDFromImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	orgMrn := orgPrefix + orgID

	model, err := r.client.GetSecurityModel(ctx, orgMrn)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import organization SLAs", err.Error())
		return
	}
	if model == nil || model.SlasSource != slaConfigurationSourceOrganization {
		resp.Diagnostics.AddError(
			"Organization has no SLAs to import",
			fmt.Sprintf("The organization %q doesn't set SLAs for its spaces, so there is nothing to import. "+
				"Create a mondoo_organization_sla resource for it instead.", orgID),
		)
		return
	}

	data := OrganizationSlaResourceModel{
		OrgID:  types.StringValue(orgID),
		OrgMrn: types.StringValue(orgMrn),
	}
	resp.Diagnostics.Append(flattenOrganizationSLAs(model.Slas, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// organizationIDFromImportID returns the organization ID from an import ID
// that is either the ID itself or the organization's MRN.
func organizationIDFromImportID(importID string) (string, error) {
	orgID := strings.TrimPrefix(importID, orgPrefix)
	if orgID == "" || strings.ContainsAny(orgID, "/ \t\n") {
		return "", fmt.Errorf(
			"expected an organization ID, such as \"my-org-123456\", or an organization MRN, such as \"%smy-org-123456\", got %q",
			orgPrefix, importID,
		)
	}
	return orgID, nil
}

// expandOrganizationSLAs converts the model to the API input. It always
// includes all four ratings and both settings, so the API's merge acts as a
// replace.
func expandOrganizationSLAs(ctx context.Context, data *OrganizationSlaResourceModel) (mondoov1.UpdateSLAsInput, diag.Diagnostics) {
	var diags diag.Diagnostics

	findings := make([]mondoov1.UpdateFindingsSLAInput, 0, len(organizationSlaRatings))
	for _, r := range organizationSlaRatings {
		obj := data.ratingObject(r.attribute)
		if obj.IsNull() || obj.IsUnknown() {
			diags.AddAttributeError(
				path.Root(r.attribute),
				"Missing SLA",
				fmt.Sprintf("The SLA for %s findings must be known before it can be applied.", r.attribute),
			)
			continue
		}

		var rating organizationSlaRatingModel
		d := obj.As(ctx, &rating, basetypes.ObjectAsOptions{})
		diags.Append(d...)
		if d.HasError() {
			continue
		}

		findings = append(findings, mondoov1.UpdateFindingsSLAInput{
			Rating:            r.rating,
			DaysToResolve:     mondoov1.Int(rating.DaysToResolve.ValueInt32()),
			DaysBeforeWarning: mondoov1.Int(rating.DaysBeforeWarning.ValueInt32()),
		})
	}

	startDateConfig := mondoov1.SLAStartDateConfig(stringOrDefault(data.StartDateConfig, organizationSlaDefaultStartDateConfig))
	ratingSource := mondoov1.SLARatingSource(stringOrDefault(data.RatingSource, organizationSlaDefaultRatingSource))

	return mondoov1.UpdateSLAsInput{
		Findings:        findings,
		StartDateConfig: &startDateConfig,
		RatingSource:    &ratingSource,
	}, diags
}

// flattenOrganizationSLAs writes the SLAs the API returned into the model.
// It fails when a rating is missing, rather than leaving a required attribute
// null in the state.
func flattenOrganizationSLAs(slas SLAsPayload, data *OrganizationSlaResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	byRating := make(map[string]FindingsSLAPayload, len(slas.Findings))
	for _, f := range slas.Findings {
		byRating[f.Rating] = f
	}

	for _, r := range organizationSlaRatings {
		f, ok := byRating[string(r.rating)]
		if !ok {
			diags.AddError(
				"Incomplete SLAs from the API",
				fmt.Sprintf("The API returned no SLA for %s findings. Please report this issue to the provider developers.", r.attribute),
			)
			continue
		}

		obj, d := types.ObjectValue(organizationSlaRatingAttrTypes, map[string]attr.Value{
			"days_to_resolve":     types.Int32Value(f.DaysToResolve),
			"days_before_warning": types.Int32Value(f.DaysBeforeWarning),
		})
		diags.Append(d...)
		if d.HasError() {
			continue
		}
		*data.ratingObject(r.attribute) = obj
	}

	// A null setting means the API default applies.
	data.StartDateConfig = types.StringValue(nonEmptyOrDefault(slas.StartDateConfig, organizationSlaDefaultStartDateConfig))
	data.RatingSource = types.StringValue(nonEmptyOrDefault(slas.RatingSource, organizationSlaDefaultRatingSource))

	return diags
}

// stringOrDefault returns the value of v, or def when v is null, unknown or
// empty.
func stringOrDefault(v types.String, def string) string {
	if v.IsNull() || v.IsUnknown() {
		return def
	}
	return nonEmptyOrDefault(v.ValueString(), def)
}

func nonEmptyOrDefault(v string, def string) string {
	if v == "" {
		return def
	}
	return v
}
