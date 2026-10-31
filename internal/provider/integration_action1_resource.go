// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = (*integrationAction1Resource)(nil)
var _ resource.ResourceWithImportState = (*integrationAction1Resource)(nil)
var _ resource.ResourceWithConfigValidators = (*integrationAction1Resource)(nil)

func NewIntegrationAction1Resource() resource.Resource {
	return &integrationAction1Resource{}
}

type integrationAction1Resource struct {
	client *ExtendedGqlClient
}

type integrationAction1ResourceModel struct {
	SpaceID types.String `tfsdk:"space_id"`
	Mrn     types.String `tfsdk:"mrn"`
	Name    types.String `tfsdk:"name"`

	Region         types.String `tfsdk:"region"`
	OrganizationID types.String `tfsdk:"organization_id"`

	// Either an existing Action1 credential...
	CredentialMrn types.String `tfsdk:"credential_mrn"`
	// ...or an API client the resource stores as a credential of its own.
	ClientID             types.String `tfsdk:"client_id"`
	ClientSecret         types.String `tfsdk:"client_secret"`
	ManagedCredentialMrn types.String `tfsdk:"managed_credential_mrn"`
}

// usesInlineClient reports whether the configuration supplies the API client
// itself rather than an existing credential.
func (m integrationAction1ResourceModel) usesInlineClient() bool {
	return !m.ClientID.IsNull()
}

// managedCredentialName names the credential the resource creates for an inline API client.
func (m integrationAction1ResourceModel) managedCredentialName() string {
	return fmt.Sprintf("%s (Action1 %s)", m.Name.ValueString(), m.OrganizationID.ValueString())
}

func (r *integrationAction1Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_action1"
}

// ConfigValidators requires exactly one way of supplying the Action1 API client.
func (r *integrationAction1Resource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("credential_mrn"),
			path.MatchRoot("client_id"),
		),
		resourcevalidator.RequiredTogether(
			path.MatchRoot("client_id"),
			path.MatchRoot("client_secret"),
		),
	}
}

func (r *integrationAction1Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Action1 integration. Scans the Windows, macOS and Linux endpoints of one Action1 organization " +
			"by delivering the scan through Action1. Requires a Mondoo Platform release with the Action1 integration.\n\n" +
			"Supply the Action1 API client either as an existing Mondoo credential (`credential_mrn`) " +
			"or inline (`client_id` and `client_secret`), in which case the resource stores it as a credential in the " +
			"integration's space and deletes that credential with the integration.",
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
				MarkdownDescription: "Name of the integration.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The Action1 region the organization lives in: one of `" +
					strings.Join(action1Regions, "`, `") + "`. `na` is app.action1.com.",
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf(action1Regions...),
				},
			},
			"organization_id": schema.StringAttribute{
				MarkdownDescription: "The Action1 organization to scan. To scan several organizations, add one integration for each.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"credential_mrn": schema.StringAttribute{
				MarkdownDescription: "MRN of an existing Action1 credential in the integration's space. " +
					"Conflicts with `client_id` and `client_secret`.",
				Optional: true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"client_id": schema.StringAttribute{
				MarkdownDescription: "Action1 API client ID. Requires `client_secret`; conflicts with `credential_mrn`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "Action1 API client secret. Requires `client_id`; conflicts with `credential_mrn`.",
				Optional:            true,
				Sensitive:           true,
				Validators: []validator.String{
					stringvalidator.LengthAtLeast(1),
				},
			},
			"managed_credential_mrn": schema.StringAttribute{
				MarkdownDescription: "MRN of the credential the resource created for `client_id` and `client_secret`. " +
					"Empty when `credential_mrn` is used.",
				Computed: true,
				PlanModifiers: []planmodifier.String{
					managedCredentialMrnModifier{},
				},
			},
		},
	}
}

func (r *integrationAction1Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationAction1Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationAction1ResourceModel
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

	credentialMrn := data.CredentialMrn.ValueString()
	data.ManagedCredentialMrn = types.StringNull()
	if data.usesInlineClient() {
		tflog.Debug(ctx, "Creating Action1 credential")
		credentialMrn, err = r.client.CreateAction1Credential(ctx, space.MRN(), data.managedCredentialName(),
			data.ClientID.ValueString(), data.ClientSecret.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to create the Action1 credential. Got error: %s", err))
			return
		}
		data.ManagedCredentialMrn = types.StringValue(credentialMrn)
	}

	tflog.Debug(ctx, "Creating Action1 integration")
	mrn, err := r.client.CreateAction1Integration(ctx, space.MRN(), data.Name.ValueString(),
		data.Region.ValueString(), data.OrganizationID.ValueString(), credentialMrn)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to create Action1 integration. Got error: %s", err))
		r.deleteManagedCredential(ctx, data.ManagedCredentialMrn, &resp.Diagnostics)
		return
	}

	data.Mrn = types.StringValue(mrn)
	data.SpaceID = types.StringValue(space.ID())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAction1Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationAction1ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	integration, err := r.client.GetAction1Integration(ctx, data.Mrn.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to read Action1 integration. Got error: %s", err))
		return
	}

	// The API never returns the credential, so the credential attributes keep their state.
	data.applyIntegration(integration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (m *integrationAction1ResourceModel) applyIntegration(integration action1Integration) {
	o := integration.ConfigurationOptions.Action1ConfigurationOptions
	m.Mrn = types.StringValue(integration.Mrn)
	m.Name = types.StringValue(integration.Name)
	m.SpaceID = types.StringValue(Integration{Mrn: integration.Mrn}.SpaceID())
	m.Region = types.StringValue(o.Region)
	m.OrganizationID = types.StringValue(o.OrganizationId)
}

func (r *integrationAction1Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state integrationAction1ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx = tflog.SetField(ctx, "mrn", plan.Mrn.ValueString())

	managed := state.ManagedCredentialMrn
	var credentialMrn *string
	// staleCredential is a managed credential to delete once the integration no longer uses it.
	staleCredential := types.StringNull()
	// newCredential is a managed credential this update created; deleted again if the update fails.
	newCredential := types.StringNull()

	switch {
	case plan.usesInlineClient() && !managed.IsNull() && managed.ValueString() != "":
		// Same managed credential; rotate it if the API client changed.
		if !plan.ClientID.Equal(state.ClientID) || !plan.ClientSecret.Equal(state.ClientSecret) {
			tflog.Debug(ctx, "Rotating Action1 credential")
			if err := r.client.RotateAction1Credential(ctx, managed.ValueString(),
				plan.ClientID.ValueString(), plan.ClientSecret.ValueString()); err != nil {
				resp.Diagnostics.AddError("Client Error",
					fmt.Sprintf("Unable to update the Action1 credential. Got error: %s", err))
				return
			}
		}
		v := managed.ValueString()
		credentialMrn = &v
		plan.ManagedCredentialMrn = managed

	case plan.usesInlineClient():
		// Switching from credential_mrn (or an imported integration) to an inline API client.
		tflog.Debug(ctx, "Creating Action1 credential")
		v, err := r.client.CreateAction1Credential(ctx, spacePrefix+state.SpaceID.ValueString(), plan.managedCredentialName(),
			plan.ClientID.ValueString(), plan.ClientSecret.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to create the Action1 credential. Got error: %s", err))
			return
		}
		credentialMrn = &v
		newCredential = types.StringValue(v)
		plan.ManagedCredentialMrn = newCredential

	default:
		// An existing credential; a managed one from before is no longer needed.
		v := plan.CredentialMrn.ValueString()
		credentialMrn = &v
		staleCredential = managed
		plan.ManagedCredentialMrn = types.StringNull()
	}

	err := r.client.UpdateAction1Integration(ctx, plan.Mrn.ValueString(), plan.Name.ValueString(),
		plan.Region.ValueString(), plan.OrganizationID.ValueString(), credentialMrn)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to update Action1 integration. Got error: %s", err))
		r.deleteManagedCredential(ctx, newCredential, &resp.Diagnostics)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	r.deleteManagedCredential(ctx, staleCredential, &resp.Diagnostics)
}

func (r *integrationAction1Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationAction1ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to delete Action1 integration. Got error: %s", err))
		return
	}

	// A credential can only be deleted once no integration uses it.
	r.deleteManagedCredential(ctx, data.ManagedCredentialMrn, &resp.Diagnostics)
}

// deleteManagedCredential deletes a credential this resource created. Failing to
// is a warning: the integration is already in the state the plan asked for.
func (r *integrationAction1Resource) deleteManagedCredential(ctx context.Context, mrn types.String, diags interface {
	AddWarning(summary, detail string)
}) {
	if mrn.IsNull() || mrn.IsUnknown() || mrn.ValueString() == "" {
		return
	}
	if err := r.client.DeleteCredentialV2(ctx, mrn.ValueString()); err != nil && !isNotFoundError(err) {
		diags.AddWarning("Client Error",
			fmt.Sprintf("Unable to delete the Action1 credential %s; delete it in the Mondoo Console. Got error: %s",
				mrn.ValueString(), err))
	}
}

// ImportState imports an integration by MRN. The API does not return which
// credential an integration uses, so after an import set `credential_mrn` to the
// integration's credential; with `client_id` and `client_secret` instead, the
// next apply stores them as a new credential and points the integration at it.
func (r *integrationAction1Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if _, ok := r.client.ImportIntegration(ctx, req, resp); !ok {
		return
	}
	integration, err := r.client.GetAction1Integration(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to read Action1 integration. Got error: %s", err))
		return
	}
	model := integrationAction1ResourceModel{
		CredentialMrn:        types.StringNull(),
		ClientID:             types.StringNull(),
		ClientSecret:         types.StringNull(),
		ManagedCredentialMrn: types.StringNull(),
	}
	model.applyIntegration(integration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// managedCredentialMrnModifier plans managed_credential_mrn: empty without an
// inline API client, the existing credential while there is one, and unknown
// when the apply creates one.
type managedCredentialMrnModifier struct{}

func (m managedCredentialMrnModifier) Description(_ context.Context) string {
	return "Keeps the managed credential while client_id is set."
}

func (m managedCredentialMrnModifier) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (m managedCredentialMrnModifier) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.Plan.Raw.IsNull() {
		return // destroy
	}
	var clientID types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("client_id"), &clientID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	switch {
	case clientID.IsNull():
		resp.PlanValue = types.StringNull()
	case !req.StateValue.IsNull() && req.StateValue.ValueString() != "":
		resp.PlanValue = req.StateValue
	default:
		resp.PlanValue = types.StringUnknown()
	}
}
