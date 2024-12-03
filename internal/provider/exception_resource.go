package provider

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
}

func (r *exceptionResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_exception"
}

func (r *exceptionResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Set custom exceptions fot a Scope.`,
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
				MarkdownDescription: "The timestamp until the exception is valid.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(regexp.MustCompile(`[1-9][0-9][0-9]{2}-([0][1-9]|[1][0-2])-([1-2][0-9]|[0][1-9]|[3][0-1])`), "Date must be in the format 'YYYY-MM-DD'"),
				},
			},
			"justification": schema.StringAttribute{
				MarkdownDescription: "Description why the exception is required.",
				Optional:            true,
			},
			"action": schema.StringAttribute{
				MarkdownDescription: "The action to perform. Default is `SNOOZE`. Other options are `ENABLE`, `DISABLE`, `OUT_OF_SCOPE`.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("SNOOZE"),
				Validators: []validator.String{
					stringvalidator.OneOf("SNOOZE", "ENABLE", "DISABLE", "OUT_OF_SCOPE"),
				},
			},
			"check_mrns": schema.ListAttribute{
				MarkdownDescription: "List of check MRNs to set exceptions for.",
				ElementType:         types.StringType,
				Optional:            true,
			},
			"vulnerability_mrns": schema.ListAttribute{
				MarkdownDescription: "List of vulnerability MRNs to set exceptions for.",
				ElementType:         types.StringType,
				Optional:            true,
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
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *exceptionResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data exceptionResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	checks := []string{}
	data.CheckMrns.ElementsAs(ctx, &checks, false)

	vulnerabilities := []string{}
	data.VulnerabilityMrns.ElementsAs(ctx, &vulnerabilities, false)

	// Format ValidUntil to RFC3339 if provided
	var validUntilStr string
	validUntil := data.ValidUntil.ValueString()
	if validUntil != "" {
		year, month, day, err := parseDate(validUntil)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Configuration", err.Error())
			return
		}
		validUntilStr = time.Date(year, month, day, time.Now().Hour(), time.Now().Minute(), time.Now().Second(), time.Now().Nanosecond(), time.Now().Location()).Format(time.RFC3339)
	}

	// Create API call logic
	// mondoov1.ExceptionMutationAction(data.Action.ValueString())
	tflog.Debug(ctx, fmt.Sprintf("Creating exception for scope %s", data.ScopeMrn.ValueString()))
	err := r.client.ApplyException(ctx, data.ScopeMrn.ValueString(), mondoov1.ExceptionMutationAction(data.Action.ValueString()), checks, []string{}, []string{}, vulnerabilities, data.Justification.ValueStringPointer(), &validUntilStr, (*bool)(mondoov1.NewBooleanPtr(false)))
	fmt.Println("====================================")
	fmt.Println("Error:", err)
	fmt.Println("====================================")
	if err != nil {
		resp.Diagnostics.AddError("Failed to create exception", err.Error())
		return
	}

	data.ScopeMrn = types.StringValue(data.ScopeMrn.ValueString())
	data.ValidUntil = types.StringValue(validUntilStr)

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

	// Compute and validate the space
	scope, err := r.client.ComputeSpace(data.ScopeMrn)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "scope_mrn", scope.MRN())

	checks := make([]string, 0)
	checkMrns := data.CheckMrns.Elements()
	for _, check := range checkMrns {
		checks = append(checks, check.(types.String).ValueString())
	}

	vulnerabilities := make([]string, 0)
	vulnerabilityMrns := data.VulnerabilityMrns.Elements()
	for _, vulnerability := range vulnerabilityMrns {
		vulnerabilities = append(vulnerabilities, vulnerability.(types.String).ValueString())
	}

	// Format ValidUntil to RFC3339 if provided
	var validUntilStr string
	validUntil := data.ValidUntil.ValueString()
	if validUntil != "" {
		year, month, day, err := parseDate(validUntil)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Configuration", err.Error())
			return
		}
		validUntilStr = time.Date(year, month, day, time.Now().Hour(), time.Now().Minute(), time.Now().Second(), time.Now().Nanosecond(), time.Now().Location()).Format(time.RFC3339)
	}

	// Update API call logic
	tflog.Debug(ctx, fmt.Sprintf("Updating exception for scope %s", scope.MRN()))
	err = r.client.ApplyException(ctx, data.ScopeMrn.ValueString(), mondoov1.ExceptionMutationAction(data.Action.ValueString()), checks, []string{}, []string{}, vulnerabilities, data.Justification.ValueStringPointer(), &validUntilStr, (*bool)(mondoov1.NewBooleanPtr(false)))
	if err != nil {
		resp.Diagnostics.AddError("Failed to update exception", err.Error())
		return
	}

	data.ScopeMrn = types.StringValue(scope.MRN())
	data.ValidUntil = types.StringValue(validUntilStr)

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

	// Compute and validate the space
	scope, err := r.client.ComputeSpace(data.ScopeMrn)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "scope_mrn", scope.MRN())

	checks := make([]string, 0)
	checkMrns := data.CheckMrns.Elements()
	for _, check := range checkMrns {
		checks = append(checks, check.(types.String).ValueString())
	}

	vulnerabilities := make([]string, 0)
	vulnerabilityMrns := data.VulnerabilityMrns.Elements()
	for _, vulnerability := range vulnerabilityMrns {
		vulnerabilities = append(vulnerabilities, vulnerability.(types.String).ValueString())
	}

	// Delete API call logic
	tflog.Debug(ctx, fmt.Sprintf("Deleting exception for scope %s", scope.MRN()))
	err = r.client.ApplyException(ctx, data.ScopeMrn.ValueString(), mondoov1.ExceptionMutationAction("ENABLE"), checks, []string{}, []string{}, vulnerabilities, data.Justification.ValueStringPointer(), (*string)(mondoov1.NewStringPtr("")), (*bool)(mondoov1.NewBooleanPtr(false)))
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete exception", err.Error())
		return
	}
}
