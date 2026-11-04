// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var (
	_ resource.Resource                = (*OrganizationPlatformPoliciesOptOutResource)(nil)
	_ resource.ResourceWithConfigure   = (*OrganizationPlatformPoliciesOptOutResource)(nil)
	_ resource.ResourceWithImportState = (*OrganizationPlatformPoliciesOptOutResource)(nil)
)

func NewOrganizationPlatformPoliciesOptOutResource() resource.Resource {
	return &OrganizationPlatformPoliciesOptOutResource{}
}

// OrganizationPlatformPoliciesOptOutResource opts an organization out of platform-level
// policies. It is separate from mondoo_organization because only a platform admin can
// change the setting, while an organization is usually managed with a credential
// of its own.
type OrganizationPlatformPoliciesOptOutResource struct {
	client *ExtendedGqlClient
}

type OrganizationPlatformPoliciesOptOutResourceModel struct {
	OrgID  types.String `tfsdk:"org_id"`
	OrgMrn types.String `tfsdk:"org_mrn"`
}

func (r *OrganizationPlatformPoliciesOptOutResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_platform_policies_opt_out"
}

func (r *OrganizationPlatformPoliciesOptOutResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Opts a Mondoo organization out of platform-level policies.

While this resource exists, no policy assigned at platform level applies to the organization, its spaces or their assets, including policies assigned at platform level later. Policies that the organization or its spaces assign still apply. Other organizations are not affected. Destroying this resource opts the organization back in.

The opt-out covers all platform-level policies and every space in the organization. You cannot opt out of single policies, and you cannot opt out a single space.

An asset changes what it runs on its next scan.

Managing this resource requires the platform admin role. Owners of the organization cannot change this setting. Use a provider configuration with a platform admin service account, for example through a provider alias. The MONDOO_API_TOKEN, MONDOO_CONFIG_PATH and MONDOO_CONFIG_BASE64 environment variables take precedence over the provider's credentials field, so leave them unset when the alias sets credentials.`,

		Attributes: map[string]schema.Attribute{
			"org_id": schema.StringAttribute{
				MarkdownDescription: "The ID of the organization, not its MRN. Changing it opts the old organization back in and opts the new one out.",
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
		},
	}
}

func (r *OrganizationPlatformPoliciesOptOutResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *OrganizationPlatformPoliciesOptOutResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data OrganizationPlatformPoliciesOptOutResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgMrn := orgPrefix + data.OrgID.ValueString()
	tflog.Debug(ctx, "opting organization out of platform-level policies", map[string]interface{}{
		"orgMrn": orgMrn,
	})

	if err := r.client.SetOrganizationPlatformPoliciesOptOut(ctx, orgMrn, true); err != nil {
		resp.Diagnostics.AddError("Failed to opt organization out of platform-level policies", err.Error())
		return
	}

	data.OrgMrn = types.StringValue(orgMrn)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationPlatformPoliciesOptOutResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data OrganizationPlatformPoliciesOptOutResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgMrn := orgPrefix + data.OrgID.ValueString()
	optedOut, err := r.client.GetOrganizationPlatformPoliciesOptedOut(ctx, orgMrn)
	if err != nil {
		if isOrganizationNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read platform-level policy opt-out of organization", err.Error())
		return
	}

	// The organization was opted back in outside of Terraform, so the next plan opts it
	// out again.
	if !optedOut {
		tflog.Warn(ctx, "organization is no longer opted out of platform-level policies, removing from state", map[string]interface{}{
			"orgMrn": orgMrn,
		})
		resp.State.RemoveResource(ctx)
		return
	}

	data.OrgMrn = types.StringValue(orgMrn)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// Update is never called with a change: org_id requires replacement and
// org_mrn is derived from it.
func (r *OrganizationPlatformPoliciesOptOutResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data OrganizationPlatformPoliciesOptOutResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *OrganizationPlatformPoliciesOptOutResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data OrganizationPlatformPoliciesOptOutResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	orgMrn := orgPrefix + data.OrgID.ValueString()
	tflog.Debug(ctx, "opting organization back in to platform-level policies", map[string]interface{}{
		"orgMrn": orgMrn,
	})

	// An organization that is gone has nothing left to opt back in.
	err := r.client.SetOrganizationPlatformPoliciesOptOut(ctx, orgMrn, false)
	if err != nil && !isOrganizationNotFoundError(err) {
		resp.Diagnostics.AddError("Failed to opt organization back in to platform-level policies", err.Error())
	}
}

// isOrganizationNotFoundError also matches the error the API returns for a deleted
// organization, "client: organization not found", which carries no gRPC code.
func isOrganizationNotFoundError(err error) bool {
	return isNotFoundError(err) || (err != nil && strings.Contains(err.Error(), "organization not found"))
}

// ImportState accepts an organization ID or an organization MRN.
func (r *OrganizationPlatformPoliciesOptOutResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	orgID, err := orgIDFromImportID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	orgMrn := orgPrefix + orgID

	optedOut, err := r.client.GetOrganizationPlatformPoliciesOptedOut(ctx, orgMrn)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import platform-level policy opt-out of organization", err.Error())
		return
	}
	if !optedOut {
		resp.Diagnostics.AddError(
			"Organization is not opted out of platform-level policies",
			fmt.Sprintf("The organization %q is not opted out, so there is nothing to import. "+
				"Create a mondoo_organization_platform_policies_opt_out resource for it instead.", orgID),
		)
		return
	}

	data := OrganizationPlatformPoliciesOptOutResourceModel{
		OrgID:  types.StringValue(orgID),
		OrgMrn: types.StringValue(orgMrn),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// orgIDFromImportID returns the organization ID from an import ID that is either the
// ID itself or the organization's MRN.
func orgIDFromImportID(importID string) (string, error) {
	orgID := strings.TrimPrefix(importID, orgPrefix)
	if orgID == "" || strings.ContainsAny(orgID, "/ \t\n") {
		return "", fmt.Errorf(
			"expected an organization ID, such as \"my-org-123456\", or an organization MRN, such as \"%smy-org-123456\", got %q",
			orgPrefix, importID,
		)
	}
	return orgID, nil
}
