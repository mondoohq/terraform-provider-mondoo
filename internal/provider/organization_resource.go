package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"go.mondoo.com/terraform-provider-mondoo/internal/mondoovalidator"
)

var _ resource.Resource = (*organizationResource)(nil)

func NewOrganizationResource() resource.Resource {
	return &organizationResource{}
}

type organizationResource struct {
	client *ExtendedGqlClient
}

type organizationResourceModel struct {
	OrgId       types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	OrgMrn      types.String `tfsdk:"mrn"`
	Description types.String `tfsdk:"description"`
	Company     types.String `tfsdk:"company"`
}

func (r *organizationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization"
}

func (r *organizationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the space.",
				Required:            true,
				Validators: []validator.String{
					mondoovalidator.Name(),
				},
			},
			"id": schema.StringAttribute{
				MarkdownDescription: "ID of the org. Must be globally unique. If the provider has a org configured and this field is empty, the provider org is used.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
				Validators: []validator.String{
					mondoovalidator.Id(),
				},
			},
			"mrn": schema.StringAttribute{
				MarkdownDescription: "Mrn of the org.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the organization.",
				Optional:            true,
			},
			"company": schema.StringAttribute{
				MarkdownDescription: "Company name of the organization.",
				Optional:            true,
			},
		},
	}
}

func (r *organizationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *organizationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data organizationResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Create API call logic
	payload, err := r.client.CreateOrganization(
		ctx,
		data.OrgId.ValueStringPointer(),
		data.Name.ValueString(),
		data.Description.ValueStringPointer(),
		data.Company.ValueStringPointer(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create organization", err.Error())
		return
	}

	data.Name = types.StringValue(payload.Name)
	data.OrgId = types.StringValue(payload.Id)
	ctx = tflog.SetField(ctx, "org_id", data.OrgId)

	data.OrgMrn = types.StringValue(payload.Mrn)
	ctx = tflog.SetField(ctx, "org_mrn", data.OrgMrn)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *organizationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data organizationResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *organizationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data organizationResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Update API call logic
	var planOrgID string
	req.Plan.GetAttribute(ctx, path.Root("id"), &planOrgID)
	if data.OrgId.ValueString() != planOrgID {
		resp.Diagnostics.AddError("Organization ID cannot be changed", "Organization ID is immutable.")
		return
	}

	err := r.client.UpdateOrganization(
		ctx,
		data.OrgMrn.ValueString(),
		data.Name.ValueString(),
		data.Description.ValueStringPointer(),
		data.Company.ValueStringPointer(),
	)
	if err != nil {
		resp.Diagnostics.AddError("Failed to update organization", err.Error())
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *organizationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data organizationResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete API call logic
	err := r.client.DeleteOrganization(ctx, data.OrgMrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete organization", err.Error())
		return
	}
}

func (r *organizationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	mrn := "//captain.api.mondoo.app/organizations/" + req.ID
	orgPayload, err := r.client.GetOrganization(ctx, mrn)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to retrieve org. Got error: %s", err),
			)
		return
	}

	model := organizationResourceModel{
		Name:        types.StringValue(orgPayload.Name),
		OrgId:       types.StringValue(orgPayload.Id),
		OrgMrn:      types.StringValue(orgPayload.Mrn),
		Description: types.StringValue(orgPayload.Description),
		Company:     types.StringValue(orgPayload.Company),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
