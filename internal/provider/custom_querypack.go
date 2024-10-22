// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = (*customQueryPackResource)(nil)

func NewCustomQueryPackResource() resource.Resource {
	return &customQueryPackResource{}
}

// customQueryPackResource defines the resource implementation.
type customQueryPackResource struct {
	client *ExtendedGqlClient
}

// customQueryPackResourceModel describes the resource data model.
type customQueryPackResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// policy mrn
	Mrns      types.List `tfsdk:"mrns"`
	Overwrite types.Bool `tfsdk:"overwrite"`

	// the content of the policy can be defined as a string a file path or as plain text content
	Source  types.String `tfsdk:"source"`
	Content types.String `tfsdk:"content"`

	// the crc32c hash of the content
	Crc32Checksum types.String `tfsdk:"crc32c"`
}

func (r *customQueryPackResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_querypack"
}

func (r *customQueryPackResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Custom Query Pack resource",
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo Space Identifier. If it is not provided, the provider space is used.",
				Optional:            true,
			},
			"mrns": schema.ListAttribute{
				MarkdownDescription: "The Mondoo Resource Name (MRN) of the created query packs",
				ElementType:         types.StringType,
				Computed:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"content": schema.StringAttribute{
				MarkdownDescription: "Data as string to be uploaded. Must be defined if source is not. Note: The content field is marked as sensitive. To view the raw contents of the object, please define an output.",
				Computed:            true,
				Sensitive:           true,
				Optional:            true,
				Validators: []validator.String{
					// Validate only this attribute or other_attr is configured.
					stringvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("source"),
					}...),
				},
			},
			"source": schema.StringAttribute{
				MarkdownDescription: "A path to the data you want to upload. Must be defined if content is not.",
				Computed:            false,
				Optional:            true,
				Validators: []validator.String{
					// Validate only this attribute or other_attr is configured.
					stringvalidator.ExactlyOneOf(path.Expressions{
						path.MatchRoot("content"),
					}...),
				},
			},
			"overwrite": schema.BoolAttribute{
				MarkdownDescription: "If set to true, existing policies are overwritten.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"crc32c": schema.StringAttribute{
				MarkdownDescription: "Base 64 CRC32 hash of the uploaded data.",
				Computed:            true,
			},
		},
	}
}

func (r *customQueryPackResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *customQueryPackResource) getContent(data customQueryPackResourceModel) ([]byte, string, error) {
	var policyBundleData []byte
	if !data.Content.IsNull() && !data.Source.IsNull() {
		// load content from file
		content, err := os.ReadFile(data.Source.ValueString())
		if err != nil {
			return nil, "", err
		}
		policyBundleData = content
	} else {
		// use content
		policyBundleData = []byte(data.Content.ValueString())
	}

	return policyBundleData, newCrc32Checksum(policyBundleData), nil
}

func (r *customQueryPackResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data customQueryPackResourceModel

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

	if data.Content.IsNull() && data.Source.IsNull() {
		resp.Diagnostics.AddError(
			"Either content or source needs to be set",
			"Either content or source needs to be set",
		)
		return
	}

	policyBundleData, checksum, err := r.getContent(data)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read content from file "+data.Source.ValueString(),
			"Unable to read content from file "+data.Source.ValueString(),
		)
		return
	}

	// Do GraphQL request to API to create the resource
	tflog.Debug(ctx, "Creating custom querypack")
	setCustomPolicyPayload, err := r.client.SetCustomQueryPack(ctx,
		space.MRN(),
		data.Overwrite.ValueBoolPointer(),
		policyBundleData,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to store policy, got error: %s", err),
			)
		return
	}

	// Save data into Terraform state
	data.Content = types.StringValue(string(policyBundleData))
	data.Crc32Checksum = types.StringValue(checksum)
	data.SpaceID = types.StringValue(space.ID())
	data.Mrns, _ = types.ListValueFrom(ctx,
		types.StringType,
		setCustomPolicyPayload.QueryPackMrns,
	)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)

}

func (r *customQueryPackResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data customQueryPackResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	//  check if the local content has changed, if so, update the policy
	policyBundleData, checksum, err := r.getContent(data)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read content from file "+data.Source.ValueString(),
			"Unable to read content from file "+data.Source.ValueString(),
		)
		return
	}

	if data.Crc32Checksum.ValueString() != checksum {
		data.Content = types.StringValue(string(policyBundleData))
		data.Crc32Checksum = types.StringValue(checksum)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *customQueryPackResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data customQueryPackResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	//  check if the local content has changed, if so, update the policy
	policyBundleData, checksum, err := r.getContent(data)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to read content from file "+data.Source.ValueString(),
			"Unable to read content from file "+data.Source.ValueString(),
		)
		return
	}

	if data.Crc32Checksum.ValueString() != checksum {
		// ensure space id is not changed
		var stateSpaceID string
		req.State.GetAttribute(ctx, path.Root("space_id"), &stateSpaceID)

		var planSpaceID string
		req.Plan.GetAttribute(ctx, path.Root("space_id"), &planSpaceID)

		providerSpaceID := r.client.Space().ID()

		if stateSpaceID != planSpaceID || providerSpaceID != planSpaceID {
			resp.Diagnostics.AddError(
				"Space ID cannot be changed",
				"Note that the Mondoo space can be configured at the resource or provider level.",
			)
			return
		}

		// Do GraphQL request to API to update the resource.
		setCustomPolicyPayload, err := r.client.SetCustomQueryPack(ctx,
			SpaceFrom(planSpaceID).MRN(),
			data.Overwrite.ValueBoolPointer(),
			policyBundleData,
		)
		if err != nil {
			resp.Diagnostics.
				AddError("Client Error",
					fmt.Sprintf("Unable to store policy, got error: %s", err),
				)
			return
		}

		data.Content = types.StringValue(string(policyBundleData))
		data.Crc32Checksum = types.StringValue(checksum)
		data.Mrns, _ = types.ListValueFrom(ctx, types.StringType, setCustomPolicyPayload.QueryPackMrns)
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *customQueryPackResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data customQueryPackResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource
	queryPackMrns := []string{}
	data.Mrns.ElementsAs(ctx, &queryPackMrns, false)

	for _, policyMrn := range queryPackMrns {
		err := r.client.DeletePolicy(ctx, policyMrn)
		if err != nil {
			resp.Diagnostics.
				AddError("Client Error",
					fmt.Sprintf("Unable to delete query pack, got error: %s", err),
				)
			return
		}
	}
}

func (r *customQueryPackResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	mrn := req.ID
	splitMrn := strings.Split(mrn, "/")
	spaceMrn := spacePrefix + splitMrn[len(splitMrn)-3]
	spaceID := splitMrn[len(splitMrn)-3]

	if r.client.Space().ID() != "" && r.client.Space().ID() != spaceID {
		// The provider is configured to manage resources in a different space than the one the
		// resource is currently configured, we won't allow that
		resp.Diagnostics.AddError(
			"Conflict Error",
			fmt.Sprintf(
				"Unable to import integration, the provider is configured in a different space than the resource. (%s != %s)",
				r.client.Space().ID(), spaceID),
		)
		return
	}

	queryPack, err := r.client.GetPolicy(ctx, mrn, spaceMrn)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to get policy, got error: %s", err),
			)
		return
	}

	content, err := r.client.DownloadBundle(ctx, string(queryPack.Mrn))
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to download bundle, got error: %s", err),
			)
		return
	}

	mrns, _ := types.ListValueFrom(ctx, types.StringType, []string{mrn})

	model := customQueryPackResourceModel{
		SpaceID:       types.StringValue(spaceID),
		Mrns:          mrns,
		Overwrite:     types.BoolValue(false),
		Source:        types.StringPointerValue(nil),
		Content:       types.StringValue(content),
		Crc32Checksum: types.StringPointerValue(nil),
	}

	checksum := newCrc32Checksum([]byte(content))

	model.Crc32Checksum = types.StringValue(checksum)

	resp.State.Set(ctx, &model)
}
