// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var _ resource.Resource = (*ResourceContactsResource)(nil)

func NewResourceContactsResource() resource.Resource {
	return &ResourceContactsResource{}
}

// ResourceContactsResource manages contacts for a Mondoo resource (organization, space, or workspace).
type ResourceContactsResource struct {
	client *ExtendedGqlClient
}

type ResourceContactsResourceModel struct {
	ResourceMrn types.String `tfsdk:"resource_mrn"`
	Contacts    types.List   `tfsdk:"contacts"`
}

func (r *ResourceContactsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_resource_contacts"
}

func (r *ResourceContactsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages contacts for a Mondoo resource (organization, space, or workspace).

This is an authoritative resource — it manages **all** contacts for the target resource.
Setting contacts replaces any existing contacts; destroying this resource clears all contacts.

## Example Usage

` + "```hcl" + `
resource "mondoo_space" "example" {
  org_id = "my-org"
  name   = "Production"
}

resource "mondoo_team" "ops" {
  name      = "ops-team"
  scope_mrn = mondoo_space.example.mrn
  email     = "ops@example.com"
}

resource "mondoo_resource_contacts" "example" {
  resource_mrn = mondoo_space.example.mrn
  contacts     = [
    mondoo_team.ops.mrn,
    "security@example.com",
  ]
}
` + "```",

		Attributes: map[string]schema.Attribute{
			"resource_mrn": schema.StringAttribute{
				MarkdownDescription: "MRN of the resource (organization, space, or workspace) to manage contacts for.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"contacts": schema.ListAttribute{
				MarkdownDescription: "List of contacts. Each entry is an identity: user MRN, team MRN, or email address.",
				Required:            true,
				ElementType:         types.StringType,
			},
		},
	}
}

func (r *ResourceContactsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ResourceContactsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ResourceContactsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	contacts := expandContactIdentities(data.Contacts)

	_, err := r.client.SetResourceContacts(ctx, data.ResourceMrn.ValueString(), contacts)
	if err != nil {
		resp.Diagnostics.AddError("Failed to set resource contacts", err.Error())
		return
	}

	// Keep the user's configured values in state rather than the server's
	// resolved identities, since the server may resolve emails to user MRNs.

	tflog.Trace(ctx, "set resource contacts", map[string]interface{}{
		"resource_mrn": data.ResourceMrn.ValueString(),
	})

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ResourceContactsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ResourceContactsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	contacts, err := r.client.GetResourceContacts(ctx, data.ResourceMrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read resource contacts", err.Error())
		return
	}

	data.Contacts = reconcileContacts(data.Contacts, contacts)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ResourceContactsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ResourceContactsResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	contacts := expandContactIdentities(data.Contacts)

	_, err := r.client.SetResourceContacts(ctx, data.ResourceMrn.ValueString(), contacts)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update resource contacts", err.Error())
		return
	}

	// Keep the user's configured values in state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ResourceContactsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ResourceContactsResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Clear all contacts by setting an empty list
	_, err := r.client.SetResourceContacts(ctx, data.ResourceMrn.ValueString(), []mondoov1.ResourceContactInput{})
	if err != nil {
		resp.Diagnostics.AddError("Failed to clear resource contacts", err.Error())
		return
	}

	tflog.Trace(ctx, "cleared resource contacts", map[string]interface{}{
		"resource_mrn": data.ResourceMrn.ValueString(),
	})
}

func (r *ResourceContactsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resourceMrn := req.ID

	contacts, err := r.client.GetResourceContacts(ctx, resourceMrn)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import resource contacts", err.Error())
		return
	}

	model := ResourceContactsResourceModel{
		ResourceMrn: types.StringValue(resourceMrn),
		Contacts:    flattenContactIdentities(contacts),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// expandContactIdentities converts a Terraform list of strings to a slice of ResourceContactInput.
func expandContactIdentities(l types.List) []mondoov1.ResourceContactInput {
	if l.IsNull() || l.IsUnknown() {
		return []mondoov1.ResourceContactInput{}
	}

	elements := l.Elements()
	result := make([]mondoov1.ResourceContactInput, 0, len(elements))
	for _, v := range elements {
		result = append(result, mondoov1.ResourceContactInput{
			Identity: mondoov1.String(v.(types.String).ValueString()),
		})
	}
	return result
}

// flattenContactIdentities converts a slice of ResourceContactPayload to a Terraform list of identity strings.
func flattenContactIdentities(contacts []ResourceContactPayload) types.List {
	if len(contacts) == 0 {
		return types.ListValueMust(types.StringType, []attr.Value{})
	}

	elements := make([]attr.Value, 0, len(contacts))
	for _, c := range contacts {
		elements = append(elements, types.StringValue(string(c.Identity)))
	}
	return types.ListValueMust(types.StringType, elements)
}

// reconcileContacts preserves user-provided identities when the server resolves
// them to different forms (e.g., email → user MRN). It matches server contacts
// back to state values using the email field, so users can write emails in config
// even when the server stores the resolved MRN.
func reconcileContacts(stateContacts types.List, serverContacts []ResourceContactPayload) types.List {
	if len(serverContacts) == 0 {
		return types.ListValueMust(types.StringType, []attr.Value{})
	}

	// Build a lookup from server identity/email to the server contact
	// so we can match state values to server contacts.
	stateValues := stateContacts.Elements()

	// Build set of state identities for quick lookup
	stateSet := make(map[string]bool, len(stateValues))
	for _, v := range stateValues {
		stateSet[v.(types.String).ValueString()] = true
	}

	// Build mapping from email → state value (for email-to-MRN resolution)
	emailToMrn := make(map[string]string)
	for _, sc := range serverContacts {
		if sc.Email != "" {
			emailToMrn[string(sc.Email)] = string(sc.Identity)
		}
	}

	// For each server contact, try to find the user's original value
	elements := make([]attr.Value, 0, len(serverContacts))
	matched := make(map[string]bool)
	for _, sc := range serverContacts {
		identity := string(sc.Identity)
		email := string(sc.Email)

		// If the server identity matches a state value directly, keep it
		if stateSet[identity] && !matched[identity] {
			elements = append(elements, types.StringValue(identity))
			matched[identity] = true
			continue
		}

		// If the server resolved an email to a MRN, check if the original
		// email is in state
		if email != "" && stateSet[email] && !matched[email] {
			elements = append(elements, types.StringValue(email))
			matched[email] = true
			continue
		}

		// Otherwise use the server identity (new contact added externally)
		elements = append(elements, types.StringValue(identity))
	}

	return types.ListValueMust(types.StringType, elements)
}
