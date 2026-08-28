// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = (*credentialResource)(nil)
var _ resource.ResourceWithImportState = (*credentialResource)(nil)
var _ resource.ResourceWithConfigValidators = (*credentialResource)(nil)
var _ resource.ResourceWithModifyPlan = (*credentialResource)(nil)

func NewCredentialResource() resource.Resource {
	return &credentialResource{}
}

type credentialResource struct {
	client *ExtendedGqlClient
}

// credentialOptionalString maps an unset optional attribute to a nil pointer so
// the field is omitted from the GraphQL input entirely.
func credentialOptionalString(v types.String) *mondoov1.String {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return mondoov1.NewStringPtr(mondoov1.String(v.ValueString()))
}

// credentialUsageObjectType is the element type of the usages list attribute.
func credentialUsageObjectType() types.ObjectType {
	return types.ObjectType{
		AttrTypes: map[string]attr.Type{
			"mrn":     types.StringType,
			"name":    types.StringType,
			"purpose": types.StringType,
			"type":    types.StringType,
		},
	}
}

func (r *credentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_credential"
}

func (r *credentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"space_id": schema.StringAttribute{
			MarkdownDescription: "Mondoo space identifier. If there is no space ID, the provider space is used. " +
				"Changing the scope replaces the credential; if an integration references it, set " +
				"`lifecycle { create_before_destroy = true }` so the replacement is created before the old one is deleted.",
			Optional: true,
			Computed: true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
				stringplanmodifier.RequiresReplace(),
			},
			Validators: []validator.String{
				stringvalidator.ConflictsWith(path.MatchRoot("scope_mrn")),
			},
		},
		"scope_mrn": schema.StringAttribute{
			MarkdownDescription: "MRN of the space or organization that owns the credential. " +
				"Changing the scope replaces the credential; if an integration references it, set " +
				"`lifecycle { create_before_destroy = true }` so the replacement is created before the old one is deleted.",
			Optional: true,
			Computed: true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
				stringplanmodifier.RequiresReplace(),
			},
			Validators: []validator.String{
				stringvalidator.ConflictsWith(path.MatchRoot("space_id")),
			},
		},
		"name": schema.StringAttribute{
			MarkdownDescription: "Name of the credential. Renaming happens in place.",
			Required:            true,
			Validators: []validator.String{
				stringvalidator.LengthAtMost(250),
			},
		},
		"mrn": schema.StringAttribute{
			MarkdownDescription: "Credential identifier.",
			Computed:            true,
			PlanModifiers: []planmodifier.String{
				stringplanmodifier.UseStateForUnknown(),
			},
		},
		"owner_mrn": schema.StringAttribute{
			MarkdownDescription: "MRN of the space or organization that owns the credential.",
			Computed:            true,
		},
		"kind": schema.StringAttribute{
			MarkdownDescription: "The kind of secret the credential holds. Derived from which kind attribute is " +
				"set; never an input. A credential's kind cannot be changed after creation.",
			Computed: true,
		},
		"health_status": schema.StringAttribute{
			MarkdownDescription: "Result of the last health probe: `UNKNOWN`, `HEALTHY`, `INVALID`, `ERROR` or `EXPIRED`.",
			Computed:            true,
		},
		"health_checked_at": schema.StringAttribute{
			MarkdownDescription: "When the credential was last probed. Null if it never was.",
			Computed:            true,
		},
		"health_error": schema.StringAttribute{
			MarkdownDescription: "Error reported by the last health probe.",
			Computed:            true,
		},
		"expires_at": schema.StringAttribute{
			MarkdownDescription: "When the secret expires, as reported by the provider. Read-only: there is no " +
				"mutation that changes an expiry in place.",
			Computed: true,
		},
		"created_at": schema.StringAttribute{
			MarkdownDescription: "When the credential was created.",
			Computed:            true,
		},
		"updated_at": schema.StringAttribute{
			MarkdownDescription: "When the credential was last updated.",
			Computed:            true,
		},
		"created_by": schema.StringAttribute{
			MarkdownDescription: "Who created the credential.",
			Computed:            true,
		},
		"field_values": schema.MapAttribute{
			MarkdownDescription: "The credential's non-secret fields as stored, including values the server " +
				"derived from the secret. Never contains a secret.",
			Computed:    true,
			ElementType: types.StringType,
		},
		"usages": schema.ListAttribute{
			MarkdownDescription: "Integration slots filled by this credential. One entry per slot, not per " +
				"integration: an integration filling two slots appears twice.",
			Computed:    true,
			ElementType: credentialUsageObjectType(),
		},
	}

	// One Optional attribute per credential kind, generated from the SDK.
	for name, attribute := range credentialSecretAttributes() {
		attrs[name] = attribute
	}

	resp.Schema = schema.Schema{
		MarkdownDescription: "A typed credential, stored once and referenced by integrations through " +
			"`credential_mrn`.\n\n" +
			"The secret is write-only: no read returns it, so the provider carries the configured value " +
			"forward and does not detect drift on it. Exactly one kind attribute must be set, and the kind " +
			"cannot be changed after creation.",
		Attributes: attrs,
	}
}

func (r *credentialResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(credentialSecretPaths()...),
	}
}

func (r *credentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

// ModifyPlan refuses a kind change at plan time.
//
// RequiresReplace would be the idiomatic answer for an immutable attribute, but
// it plans destroy-then-create, and deleting a credential an integration
// references is refused server-side — so the plan would fail partway through
// apply. A hard error with the migration recipe is the honest alternative.
func (r *credentialResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	// Skip create (no state) and destroy (no plan): every attribute
	// transitions by definition in both.
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}

	var state, plan credentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	stateKind, err := state.SecretKind()
	if err != nil {
		// Nothing to compare against — an imported credential, say. A
		// malformed configuration is ExactlyOneOf's to report.
		return
	}
	planKind, err := plan.SecretKind()
	if err != nil {
		return
	}

	if stateKind != planKind {
		resp.Diagnostics.AddError(
			"Credential kind cannot be changed",
			fmt.Sprintf(
				"This credential is of kind %s and cannot be rotated to kind %s. Only renaming and rotating "+
					"the secret of the same kind are supported.\n\n"+
					"To migrate, declare the new credential alongside the old one, re-point every integration "+
					"at the new MRN, then remove the old resource. Terraform applies all three in a single run.",
				stateKind, planKind,
			),
		)
	}
}

// applyCredential copies a server response into the model, leaving the secret
// attributes untouched — the secret is write-only and is carried from config.
func (r *credentialResource) applyCredential(ctx context.Context, cred CredentialV2, data *credentialResourceModel, diags *diag.Diagnostics) {
	data.Mrn = types.StringValue(cred.Mrn)
	data.Name = types.StringValue(cred.Name)
	data.OwnerMrn = types.StringValue(cred.OwnerMrn)
	data.Kind = types.StringValue(cred.Kind)
	data.HealthStatus = types.StringValue(cred.HealthStatus)
	data.HealthCheckedAt = types.StringPointerValue(cred.HealthCheckedAt)
	data.HealthError = types.StringPointerValue(cred.HealthError)
	data.ExpiresAt = types.StringPointerValue(cred.ExpiresAt)
	data.CreatedAt = types.StringValue(cred.CreatedAt)
	data.UpdatedAt = types.StringValue(cred.UpdatedAt)
	data.CreatedBy = types.StringPointerValue(cred.CreatedBy)

	fieldValues, d := types.MapValueFrom(ctx, types.StringType, cred.FieldValuesMap())
	diags.Append(d...)
	data.FieldValues = fieldValues

	usages := make([]attr.Value, 0, len(cred.Usages))
	for _, u := range cred.Usages {
		obj, d := types.ObjectValue(credentialUsageObjectType().AttrTypes, map[string]attr.Value{
			"mrn":     types.StringValue(u.IntegrationUsage.Mrn),
			"name":    types.StringValue(u.IntegrationUsage.Name),
			"purpose": types.StringValue(u.IntegrationUsage.Purpose),
			"type":    types.StringValue(u.IntegrationUsage.Type),
		})
		diags.Append(d...)
		usages = append(usages, obj)
	}
	usageList, d := types.ListValue(credentialUsageObjectType(), usages)
	diags.Append(d...)
	data.Usages = usageList
}

func (r *credentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data credentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A credential is owned by a space or an organization. scope_mrn wins when
	// set; otherwise space_id, falling back to the provider-level space.
	scopeMrn := data.ScopeMrn.ValueString()
	if scopeMrn == "" {
		space, err := r.client.ComputeSpace(data.SpaceID)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Configuration", err.Error())
			return
		}
		scopeMrn = space.MRN()
		data.SpaceID = types.StringValue(space.ID())
	} else if data.SpaceID.IsUnknown() {
		data.SpaceID = types.StringNull()
	}
	ctx = tflog.SetField(ctx, "scope_mrn", scopeMrn)

	secret, err := data.BuildSecretInput()
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}

	tflog.Debug(ctx, "Creating credential")
	cred, err := r.client.CreateCredentialV2(ctx, scopeMrn, data.Name.ValueString(), secret)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create credential. Got error: %s", err))
		return
	}

	r.applyCredential(ctx, cred, &data, &resp.Diagnostics)
	if data.ScopeMrn.IsNull() || data.ScopeMrn.IsUnknown() {
		data.ScopeMrn = types.StringValue(cred.OwnerMrn)
	}
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *credentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data credentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// A real read, unlike integration_slack_resource.go's deliberate no-op: a
	// credential has genuinely readable state. The secret attributes are left
	// exactly as they are — they are carried forward from prior state and never
	// queried.
	cred, err := r.client.GetCredentialV2(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read credential. Got error: %s", err))
		return
	}
	if cred == nil {
		// Deleted outside Terraform.
		resp.State.RemoveResource(ctx)
		return
	}

	r.applyCredential(ctx, *cred, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *credentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state credentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mrn := state.Mrn.ValueString()
	var (
		cred    CredentialV2
		updated bool
	)

	if !plan.Name.Equal(state.Name) {
		renamed, err := r.client.RenameCredentialV2(ctx, mrn, plan.Name.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to rename credential. Got error: %s", err))
			return
		}
		cred, updated = renamed, true
	}

	// The unit of change is the whole kind attribute, never a single field:
	// rotateCredentialV2 replaces the payload rather than merging it.
	planSecret, err := plan.BuildSecretInput()
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	// stateErr != nil means prior state carries no secret — an imported
	// credential — so the configured secret has never been sent and must be.
	stateSecret, stateErr := state.BuildSecretInput()
	if stateErr != nil || !secretInputsEqual(planSecret, stateSecret) {
		rotated, rotErr := r.client.RotateCredentialV2(ctx, mrn, planSecret)
		if rotErr != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to rotate credential. Got error: %s", rotErr))
			return
		}
		cred, updated = rotated, true
	}

	if !updated {
		fetched, err := r.client.GetCredentialV2(ctx, mrn)
		if err != nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read credential. Got error: %s", err))
			return
		}
		if fetched == nil {
			resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Credential %q no longer exists.", mrn))
			return
		}
		cred = *fetched
	}

	r.applyCredential(ctx, cred, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *credentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data credentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mrn := data.Mrn.ValueString()
	if _, err := r.client.DeleteCredentialV2(ctx, mrn); err != nil {
		// Delete is refused while any integration references the credential.
		// Name them and the slot each fills rather than echoing the raw error.
		if cred, readErr := r.client.GetCredentialV2(ctx, mrn); readErr == nil && cred != nil && len(cred.Usages) > 0 {
			resp.Diagnostics.AddError(
				"Credential is in use",
				fmt.Sprintf(
					"Cannot delete credential %q while it is referenced by:%s\n\n"+
						"Remove or re-point those integrations first. Deleting the references too would "+
						"leave the integrations with no secret at all.",
					cred.Name, credentialUsageSummary(cred.Usages),
				),
			)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete credential. Got error: %s", err))
		return
	}
}

func (r *credentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	cred, err := r.client.GetCredentialV2(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to import credential %q. Got error: %s", req.ID, err))
		return
	}
	if cred == nil {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("No credential with MRN %q.", req.ID))
		return
	}

	// The secret cannot be imported: no read returns it. The kind attribute is
	// left null and the next plan proposes a rotation from configuration.
	data := credentialResourceModel{
		SpaceID:  types.StringNull(),
		ScopeMrn: types.StringValue(cred.OwnerMrn),
	}
	r.applyCredential(ctx, *cred, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// secretInputsEqual compares two secret payloads by value. Both come from the
// same generated builder, so comparing the marshalled form is exact and needs
// no per-kind knowledge.
func secretInputsEqual(a, b mondoov1.CredentialV2SecretInput) bool {
	aj, aerr := json.Marshal(a)
	bj, berr := json.Marshal(b)
	if aerr != nil || berr != nil {
		return false
	}
	return bytes.Equal(aj, bj)
}
