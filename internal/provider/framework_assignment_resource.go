package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = (*frameworkAssignmentResource)(nil)

func NewFrameworkAssignmentResource() resource.Resource {
	return &frameworkAssignmentResource{}
}

type frameworkAssignmentResource struct {
	client *ExtendedGqlClient
}

type frameworkAssignmentResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// resource details
	FrameworkMrn types.List `tfsdk:"framework_mrn"`
	Enabled      types.Bool `tfsdk:"enabled"`
}

func (r *frameworkAssignmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_framework_assignment"
}

func (r *frameworkAssignmentResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Set Compliance Frameworks for a Mondoo Space.`,
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo Space Identifier. If it is not provided, the provider space is used.",
				Optional:            true,
			},
			"framework_mrn": schema.ListAttribute{
				MarkdownDescription: "Compliance Framework MRN.",
				Required:            true,
				ElementType:         types.StringType,
			},
			"enabled": schema.BoolAttribute{
				MarkdownDescription: "Enable or disable the Compliance Framework.",
				Required:            true,
			},
		},
	}
}

func (r *frameworkAssignmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *frameworkAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data frameworkAssignmentResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Compute and validate the space
	space, err := r.client.ComputeSpace(data.SpaceID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid Configuration", err.Error())
		return
	}
	ctx = tflog.SetField(ctx, "space_mrn", space.MRN())

	// Do GraphQL request to API to create the resource.
	tflog.Debug(ctx, "Creating framework assignment")
	err = r.client.BulkUpdateFramework(ctx,
		data.FrameworkMrn,
		space.ID(),
		data.Enabled.ValueBool(),
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create Compliance Framework, got error: %s", err),
			)
		return
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *frameworkAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data frameworkAssignmentResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *frameworkAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data frameworkAssignmentResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// ensure space id is not changed
	var stateSpaceID string
	req.State.GetAttribute(ctx, path.Root("id"), &stateSpaceID)

	var planSpaceID string
	req.Plan.GetAttribute(ctx, path.Root("id"), &planSpaceID)

	providerSpaceID := r.client.Space().ID()

	if stateSpaceID != planSpaceID || providerSpaceID != planSpaceID {
		resp.Diagnostics.AddError(
			"Space ID cannot be changed",
			"Note that the Mondoo space can be configured at the resource or provider level.",
		)
		return
	}

	tflog.Debug(ctx, "Updating framework assignment")
	err := r.client.BulkUpdateFramework(ctx,
		data.FrameworkMrn,
		planSpaceID,
		data.Enabled.ValueBool())
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create Compliance Framework, got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *frameworkAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data frameworkAssignmentResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.BulkUpdateFramework(ctx,
		data.FrameworkMrn,
		data.SpaceID.ValueString(),
		false)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create Compliance Framework, got error: %s", err),
			)
		return
	}
}

func (r *frameworkAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("mrn"), req, resp)
}
