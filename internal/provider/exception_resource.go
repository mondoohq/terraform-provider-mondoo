package provider

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var _ resource.Resource = (*exceptionResource)(nil)

func NewExceptionResource() resource.Resource {
	return &exceptionResource{}
}

// parseDate parses a date string in the format "YYYY-MM-DD" and returns the year, month, and day as integers.
func parseDate(dateStr string) (int, time.Month, int, error) {
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return 0, 0, 0, err
	}
	return t.Year(), t.Month(), t.Day(), nil
}

type exceptionResource struct {
	client *ExtendedGqlClient
}

type exceptionResourceModel struct {
	ScopeMrn          types.String `tfsdk:"scope_mrn"`
	ValidUntil        types.String `tfsdk:"valid_until"`
	Justification     types.String `tfsdk:"justification"`
	Action            types.String `tfsdk:"action"`
	CheckMrns         types.List   `tfsdk:"check_mrns"`
	VulnerabilityMrns types.List   `tfsdk:"vulnerability_mrns"`
	ExceptionId       types.String `tfsdk:"exception_id"`
}

func (r *exceptionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_exception"
}

func (r *exceptionResource) GetConfigurationOptions(ctx context.Context, data *exceptionResourceModel) (scopeMrn string, checks []string, vulnerabilities []string, validUntilStr string, err error) {
	// Extract ScopeMrn
	scopeMrn = data.ScopeMrn.ValueString()
	if scopeMrn == "" {
		scopeMrn = r.client.space.MRN()
	}

	// Extract Checks and Vulnerabilities
	checks = []string{}
	data.CheckMrns.ElementsAs(ctx, &checks, false)

	vulnerabilities = []string{}
	data.VulnerabilityMrns.ElementsAs(ctx, &vulnerabilities, false)

	// Format ValidUntil to RFC3339 if provided
	validUntil := data.ValidUntil.ValueString()
	if validUntil != "" {
		year, month, day, parseErr := parseDate(validUntil)
		if parseErr != nil {
			return "", nil, nil, "", parseErr
		}
		now := time.Now().UTC() // Use UTC directly
		validUntilStr = time.Date(
			year,
			month,
			day,
			now.Hour(),
			now.Minute(),
			now.Second(),
			now.Nanosecond(),
			time.UTC,
		).Format(time.RFC3339Nano) // Use RFC3339Nano to include nanoseconds
	}

	return scopeMrn, checks, vulnerabilities, validUntilStr, nil
}

// ValidUntilActionValidator ensures the "valid_until" attribute is only set when "action" is "SNOOZE", "RISK_ACCEPTED", "WORKAROUND" or "FALSE_POSITIVE".
type ValidUntilActionValidator struct{}

// NewValidUntilActionValidator is a convenience function for creating an instance of the validator.
func NewValidUntilActionValidator() validator.String {
	return &ValidUntilActionValidator{}
}

// ValidateString performs the validation for the "valid_until" attribute.
func (v ValidUntilActionValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// Retrieve the "action" attribute value from the attribute path
	var actionAttr types.String
	err := req.Config.GetAttribute(ctx, path.Root("action"), &actionAttr)
	if err != nil || actionAttr.IsNull() {
		return // If "action" is not set or there's an error, nothing to validate
	}

	validUntilActions := []string{"RISK_ACCEPTED", "WORKAROUND", "FALSE_POSITIVE", "SNOOZE"}
	if !slices.Contains(validUntilActions, actionAttr.ValueString()) && !req.ConfigValue.IsNull() {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"'valid_until' Can Only Be Set with 'action' as `SNOOZE`, 'RISK_ACCEPTED', 'WORKAROUND' or 'FALSE_POSITIVE'",
			"To use 'valid_until', the 'action' attribute must be set to one of the above. Either remove 'valid_until' or change 'action'.",
		)
	}
}

// Description returns a plain-text description of the validator's purpose.
func (v ValidUntilActionValidator) Description(ctx context.Context) string {
	return "'valid_until' can only be set if 'action' is set to 'SNOOZE', 'RISK_ACCEPTED', 'WORKAROUND' or 'FALSE_POSITIVE'."
}

// MarkdownDescription returns a markdown-formatted description of the validator's purpose.
func (v ValidUntilActionValidator) MarkdownDescription(ctx context.Context) string {
	return "'valid_until' can only be set if 'action' is set to 'SNOOZE', 'RISK_ACCEPTED', 'WORKAROUND' or 'FALSE_POSITIVE'."
}

// ValidUntilPresentValidator ensures the "valid_until" attribute is only set when "action" is "SNOOZE", "RISK_ACCEPTED", "WORKAROUND" or "FALSE_POSITIVE".
type ValidUntilPresentValidator struct{}

// NewValidUntilPresentValidator is a convenience function for creating an instance of the validator.
func NewValidUntilPresentValidator() validator.String {
	return &ValidUntilPresentValidator{}
}

// ValidateString performs the validation for the "valid_until" attribute.
func (v ValidUntilPresentValidator) ValidateString(ctx context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	// Retrieve the "action" attribute value from the attribute path
	var actionAttr types.String
	err := req.Config.GetAttribute(ctx, path.Root("action"), &actionAttr)
	if err != nil || actionAttr.IsNull() {
		return // If "action" is not set or there's an error, nothing to validate
	}

	validUntilActions := []string{"RISK_ACCEPTED", "WORKAROUND", "FALSE_POSITIVE", "SNOOZE"}

	if slices.Contains(validUntilActions, actionAttr.ValueString()) && req.ConfigValue.IsNull() {
		resp.Diagnostics.AddAttributeError(
			req.Path,
			"'valid_until' Must be supplied when 'actions' is 'SNOOZE', 'RISK_ACCEPTED', 'WORKAROUND' or 'FALSE_POSITIVE'",
			fmt.Sprintf("'valid_until' Must be supplied when 'actions' is %s", actionAttr.ValueString()),
		)
	}
}

// Description returns a plain-text description of the validator's purpose.
func (v ValidUntilPresentValidator) Description(ctx context.Context) string {
	return "'valid_until' must be supplied when 'actions' is 'SNOOZE', 'RISK_ACCEPTED', 'WORKAROUND' or 'FALSE_POSITIVE'"
}

// MarkdownDescription returns a markdown-formatted description of the validator's purpose.
func (v ValidUntilPresentValidator) MarkdownDescription(ctx context.Context) string {
	return "'valid_until' must be supplied when 'actions' is 'SNOOZE', 'RISK_ACCEPTED', 'WORKAROUND' or 'FALSE_POSITIVE'"
}

func (r *exceptionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Set custom exceptions for a scope.`,
		Attributes: map[string]schema.Attribute{
			"scope_mrn": schema.StringAttribute{
				MarkdownDescription: "The MRN of the scope (either asset mrn or space mrn).",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"valid_until": schema.StringAttribute{
				MarkdownDescription: "The date when the exception is no longer valid.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`[1-9][0-9][0-9]{2}-([0][1-9]|[1][0-2])-([1-2][0-9]|[0][1-9]|[3][0-1])`), "Date must be in the format 'YYYY-MM-DD'"),
					NewValidUntilActionValidator(),
					NewValidUntilPresentValidator(),
				},
			},
			"justification": schema.StringAttribute{
				MarkdownDescription: "Description why the exception is required.",
				Optional:            true,
			},
			"action": schema.StringAttribute{
				MarkdownDescription: "The action to perform. Default is `RISK_ACCEPTED`. Other valid values are `WORKAROUND`, `FALSE_POSITIVE`, `ENABLE`, `DISABLE`, `OUT_OF_SCOPE` and `SNOOZE`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("RISK_ACCEPTED"),
				Validators: []validator.String{
					stringvalidator.OneOf("SNOOZE", "RISK_ACCEPTED", "FALSE_POSITIVE", "WORKAROUND", "ENABLE", "DISABLE", "OUT_OF_SCOPE"),
				},
			},
			"check_mrns": schema.ListAttribute{
				MarkdownDescription: "List of check MRNs to set exceptions for. If set, `vulnerability_mrns` must not be set.",
				ElementType:         types.StringType,
				Optional:            true,
				Validators: []validator.List{
					listvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot("vulnerability_mrns"),
					}...),
					listvalidator.ExactlyOneOf(path.MatchRoot("check_mrns"), path.MatchRoot("vulnerability_mrns")),
				},
			},
			"vulnerability_mrns": schema.ListAttribute{
				MarkdownDescription: "List of vulnerability MRNs to set exceptions for. If set, `check_mrns` must not be set.",
				ElementType:         types.StringType,
				Optional:            true,
				Validators: []validator.List{
					listvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot("check_mrns"),
					}...),
					listvalidator.ExactlyOneOf(path.MatchRoot("check_mrns"), path.MatchRoot("vulnerability_mrns")),
				},
			},
			"exception_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the exception",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *exceptionResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *exceptionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data exceptionResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if data.Action.ValueString() == "SNOOZE" {
		resp.Diagnostics.AddWarning(
			"use of deprecated exception action",
			`exception action 'SNOOZE' is deprecated, please use 'RISK_ACCEPTED', 'WORKAROUND' OR 'FALSE_POSITIVE'`,
		)
	}

	if resp.Diagnostics.HasError() {
		return
	}

	scopeMrn, checks, vulnerabilities, validUntilStr, err := r.GetConfigurationOptions(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}

	// Create API call logic
	tflog.Debug(ctx, fmt.Sprintf("Creating exception for scope %s", data.ScopeMrn.ValueString()))
	id, err := r.client.CreateException(ctx, scopeMrn, mondoov1.ExceptionMutationAction(data.Action.ValueString()), checks, []string{}, []string{}, vulnerabilities, data.Justification.ValueStringPointer(), &validUntilStr, (*bool)(mondoov1.NewBooleanPtr(false)))
	if err != nil {
		resp.Diagnostics.AddError("Failed to create exception", err.Error())
		return
	}
	data.ExceptionId = types.StringValue(id)
	data.ScopeMrn = types.StringValue(scopeMrn)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *exceptionResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data exceptionResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *exceptionResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data exceptionResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, checks, vulnerabilities, validUntilStr, err := r.GetConfigurationOptions(ctx, &data)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	exceptionId := data.ExceptionId.ValueString()
	if data.ExceptionId.IsNull() || data.ExceptionId.ValueString() == "" {
		tflog.Debug(ctx, "No exception ID found in state, searching for existing exception")
		// list the exceptions using data from the state
		finding, findingType := getFindingType(data)
		res, err := r.client.FindException(ctx, data.ScopeMrn.ValueString(), finding, findingType)
		if err != nil {
			// warn the user that the exception was not found. instruct them to import the exception
			resp.Diagnostics.AddError("Failed to find existing exception. Please import the exception.", err.Error())
			return
		}
		fmt.Printf("Found exception ID: %s\n", res.ExceptionID)
		// if we find an exception, set the exception id on the data
		data.ExceptionId = types.StringValue(res.ExceptionID)
		exceptionId = res.ExceptionID
	}

	if exceptionId != "" {
		tflog.Debug(ctx, fmt.Sprintf("Deleting exception for scope %s", data.ScopeMrn.ValueString()))
		err = r.client.DeleteExceptions(ctx, []string{exceptionId}, data.ScopeMrn.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to disable existing exception", err.Error())
			return
		}
	}

	if data.Action.ValueString() == "SNOOZE" {
		resp.Diagnostics.AddWarning(
			"use of deprecated exception action",
			`exception action 'SNOOZE' is deprecated, please use 'RISK_ACCEPTED', 'WORKAROUND' OR 'FALSE_POSITIVE'`,
		)
	}

	// Update API call logic
	tflog.Debug(ctx, fmt.Sprintf("Creating exception for scope %s", data.ScopeMrn.ValueString()))
	_, err = r.client.CreateException(ctx, data.ScopeMrn.ValueString(), mondoov1.ExceptionMutationAction(data.Action.ValueString()), checks, []string{}, []string{}, vulnerabilities, data.Justification.ValueStringPointer(), &validUntilStr, (*bool)(mondoov1.NewBooleanPtr(false)))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update exception", err.Error())
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *exceptionResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data exceptionResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}
	exceptionId := data.ExceptionId.ValueString()
	if data.ExceptionId.IsNull() || data.ExceptionId.ValueString() == "" {
		tflog.Debug(ctx, "No exception ID found in state, searching for existing exception")
		// list the exceptions using data from the state
		finding, findingType := getFindingType(data)
		res, err := r.client.FindException(ctx, data.ScopeMrn.ValueString(), finding, findingType)
		if err != nil {
			resp.Diagnostics.AddError("Failed to find existing exception. Please import the exception.", err.Error())
			return
		}
		tflog.Debug(ctx, fmt.Sprintf("Found exception ID: %s", res.ExceptionID))
		// if we find an exception, set the exception id on the data
		data.ExceptionId = types.StringValue(res.ExceptionID)
		exceptionId = res.ExceptionID
	}

	// Delete API call logic
	tflog.Debug(ctx, fmt.Sprintf("Deleting exception %s for scope %s", exceptionId, data.ScopeMrn.ValueString()))
	err := r.client.DeleteExceptions(ctx, []string{exceptionId}, data.ScopeMrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete exception", err.Error())
		return
	}
}

func getFindingType(data exceptionResourceModel) (string, mondoov1.ExceptionType) {
	if len(data.CheckMrns.Elements()) > 0 {
		var checks []string
		data.CheckMrns.ElementsAs(context.Background(), &checks, false)
		if len(checks) > 0 {
			return checks[0], mondoov1.ExceptionTypeSecurity
		}
	}
	if len(data.VulnerabilityMrns.Elements()) > 0 {
		var vulnerabilities []string
		data.VulnerabilityMrns.ElementsAs(context.Background(), &vulnerabilities, false)
		if len(vulnerabilities) > 0 {
			return vulnerabilities[0], mondoov1.ExceptionTypeCve
		}
	}
	return "", ""
}
func (r *exceptionResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var data exceptionResourceModel

	// Read the import ID into the model
	data.ExceptionId = types.StringValue(req.ID)

	exception, ok := r.client.ImportException(ctx, req, resp, r.client.space.MRN())
	if !ok {
		resp.Diagnostics.AddError("Failed to import exception", "Please check the import ID and try again.")
		return
	}
	// set the state with the imported exception data
	data.ScopeMrn = types.StringValue(exception.ScopeMrn)
	if exception.ValidUntil != nil {
		t, _ := time.Parse(time.RFC3339, *exception.ValidUntil)
		st := t.UTC().Format(time.DateOnly) // Ensure the date is parsed correctly
		data.ValidUntil = types.StringValue(st)
	}
	if exception.Justification != nil {
		data.Justification = types.StringValue(*exception.Justification)
	}
	data.Action = types.StringValue(exception.Action)
	checkMrns := make(map[string]bool)
	vulnMrns := make(map[string]bool)
	advisoryMrns := make(map[string]bool)
	if len(exception.Exceptions) > 0 {
		for _, mrn := range exception.Exceptions {
			// @vj: i dont understand why the items are being marshalled into both structs,
			// but they seem to be, so im filtering by the mrn prefix to ensure we dont double up
			if strings.HasPrefix(mrn.CheckMrns.Mrn, "//policy.api.mondoo.app/queries") {
				checkMrns[mrn.CheckMrns.Mrn] = true
			} else if strings.HasPrefix(mrn.VulnerabilityMrns.Mrn, "//vadvisor.api.mondoo.app/cves") {
				vulnMrns[mrn.VulnerabilityMrns.Mrn] = true
			} else if strings.HasPrefix(mrn.AdvisoryMrns.Mrn, "//vadvisor.api.mondoo.app/advisories") {
				advisoryMrns[mrn.AdvisoryMrns.Mrn] = true
			}
		}
	}
	if len(checkMrns) > 0 {
		l := slices.Collect(maps.Keys((checkMrns)))
		sort.Strings(l)
		data.CheckMrns = ConvertListValue(l)
	} else {
		data.CheckMrns = ConvertListValue([]string{})
	}
	if len(vulnMrns) > 0 {
		l := slices.Collect(maps.Keys((vulnMrns)))
		sort.Strings(l)
		data.VulnerabilityMrns = ConvertListValue(l)
	} else {
		data.VulnerabilityMrns = ConvertListValue([]string{})
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
