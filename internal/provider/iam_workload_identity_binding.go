// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int32planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var _ resource.Resource = (*IAMWorkloadIdentityBindingResource)(nil)

func NewIAMWorkloadIdentityBindingResource() resource.Resource {
	return &IAMWorkloadIdentityBindingResource{}
}

// IAMWorkloadIdentityBindingResource defines the resource implementation.
type IAMWorkloadIdentityBindingResource struct {
	client *ExtendedGqlClient
}

// IAMWorkloadIdentityBindingResourceModel describes the resource data model.
type IAMWorkloadIdentityBindingResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// Binding details

	// Mondoo resource name
	Mrn types.String `tfsdk:"mrn"`
	// User selected name. (Required.)
	Name types.String `tfsdk:"name"`
	// URI for the token issuer, e.g. https://accounts.google.com. (Required.)
	IssuerURI types.String `tfsdk:"issuer_uri"`
	// Optional description. (Optional.)
	Description types.String `tfsdk:"description"`
	// List of roles associated with the binding (e.g. agent mrn). (Optional.)
	Roles types.List `tfsdk:"roles"`
	// Unique identifier to confirm. (Required.)
	Subject types.String `tfsdk:"subject"`
	// Expiration in seconds associated with the binding. (Optional.)
	Expiration types.Int32 `tfsdk:"expiration"`
	// List of allowed audiences. (Optional.)
	AllowedAudiences types.List `tfsdk:"allowed_audiences"`
	// List of additional configurations to confirm. (Optional.)
	Mappings types.Map `tfsdk:"mappings"`
}

func (r *IAMWorkloadIdentityBindingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_iam_workload_identity_binding"
}

func (r *IAMWorkloadIdentityBindingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: `Allows management of a Mondoo Workload Identity Federation bindings.`,

		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo space identifier. If there is no ID, the provider space is used.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mrn": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The Mondoo resource name (MRN) of the created binding.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the binding.",
				Required:            true,
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Description of the binding.",
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("Created by Terraform"),
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"roles": schema.ListAttribute{
				MarkdownDescription: "List of roles associated with the binding (e.g. agent mrn).",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"issuer_uri": schema.StringAttribute{
				MarkdownDescription: "URI for the token issuer, e.g. https://accounts.google.com.",
				Required:            true,
			},
			"subject": schema.StringAttribute{
				MarkdownDescription: "Unique identifier to confirm.",
				Required:            true,
			},
			"expiration": schema.Int32Attribute{
				MarkdownDescription: "Expiration in seconds associated with the binding.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.Int32{
					int32planmodifier.UseStateForUnknown(),
				},
			},
			"allowed_audiences": schema.ListAttribute{
				MarkdownDescription: " List of allowed audiences.",
				ElementType:         types.StringType,
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.List{
					listplanmodifier.UseStateForUnknown(),
				},
			},
			"mappings": schema.MapAttribute{
				MarkdownDescription: "List of additional configurations to confirm.",
				Optional:            true,
				ElementType:         types.StringType,
				PlanModifiers: []planmodifier.Map{
					mapplanmodifier.UseStateForUnknown(),
				},
			},
		},
	}
}

func (r *IAMWorkloadIdentityBindingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

type WIFAuthBinding struct {
	Mrn              string
	Name             string
	Description      string
	Scope            string
	Roles            []string
	Expiration       int32
	IssuerURI        string
	Subject          string
	Mappings         []KeyValue
	AllowedAudiences []string
}

type WIFExternalAuthConfig struct {
	UniverseDomain   string
	Type             string
	Audience         string
	SubjectTokenType string
	Scopes           []string
	IssuerURI        string
}

func (r *IAMWorkloadIdentityBindingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data IAMWorkloadIdentityBindingResourceModel

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

	// Do GraphQL request to API to create the resource
	var (
		roles            = ConvertSliceStrings(data.Roles)
		allowedAudiences = ConvertSliceStrings(data.AllowedAudiences)
	)

	var mappings []mondoov1.KeyValueInput
	mappingsMap, d := data.Mappings.ToMapValue(context.Background())
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	for key, value := range mappingsMap.Elements() {
		mappings = append(mappings, mondoov1.KeyValueInput{
			Key:   mondoov1.String(key),
			Value: mondoov1.NewStringPtr(mondoov1.String(value.String())),
		})
	}

	createInput := mondoov1.CreateWIFAuthBindingInput{
		ScopeMrn:         mondoov1.String(space.MRN()),
		Name:             mondoov1.String(data.Name.ValueString()),
		Description:      mondoov1.NewStringPtr(mondoov1.String(data.Description.ValueString())),
		Roles:            &roles,
		IssuerURI:        mondoov1.String(data.IssuerURI.ValueString()),
		Subject:          mondoov1.String(data.Subject.ValueString()),
		AllowedAudiences: &allowedAudiences,
		Mappings:         &mappings,
	}

	if expiration := data.Expiration.ValueInt32(); expiration != 0 {
		createInput.Expiration = mondoov1.NewIntPtr(mondoov1.Int(expiration))
	}

	tflog.Debug(ctx, "CreateWIFAuthBindingInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", createInput),
	})

	var createMutation struct {
		CreateIAMWorkloadIdentityBinding struct {
			Binding WIFAuthBinding
			Config  WIFExternalAuthConfig
		} `graphql:"createWIFAuthBinding(input: $input)"`
	}

	err = r.client.Mutate(ctx, &createMutation, createInput, nil)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create binding. Got error: %s", err),
			)
		return
	}

	// Write logs using the tflog package
	tflog.Debug(ctx, "created a b2nding resource", map[string]interface{}{
		"input": fmt.Sprintf("%+v", createMutation),
	})
	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(createMutation.CreateIAMWorkloadIdentityBinding.Binding.Mrn)
	data.Description = types.StringValue(createMutation.CreateIAMWorkloadIdentityBinding.Binding.Description)
	data.Roles = ConvertListValue(createMutation.CreateIAMWorkloadIdentityBinding.Binding.Roles)
	data.AllowedAudiences = ConvertListValue(createMutation.CreateIAMWorkloadIdentityBinding.Binding.AllowedAudiences)
	data.SpaceID = types.StringValue(space.ID())
	data.Expiration = types.Int32Value(createMutation.CreateIAMWorkloadIdentityBinding.Binding.Expiration)
	if len(createMutation.CreateIAMWorkloadIdentityBinding.Binding.Mappings) != 0 {
		newMappings, _ := types.MapValueFrom(context.Background(), types.StringType, createMutation.CreateIAMWorkloadIdentityBinding.Binding.Mappings)
		data.Mappings = newMappings
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *IAMWorkloadIdentityBindingResource) readIAMWorkloadIdentityBinding(ctx context.Context, mrn string) (IAMWorkloadIdentityBindingResourceModel, error) {
	var q struct {
		IAMWorkloadIdentityBinding struct {
			Binding WIFAuthBinding
			Config  WIFExternalAuthConfig
		} `graphql:"getWIFAuthBinding(mrn: $mrn)"`
	}
	variables := map[string]interface{}{
		"mrn": mondoov1.String(mrn),
	}

	tflog.Debug(ctx, "getWIFAuthBindingInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", variables),
	})

	err := r.client.Query(ctx, &q, variables)
	if err != nil {
		return IAMWorkloadIdentityBindingResourceModel{}, err
	}

	tflog.Debug(ctx, "getWIFAuthBindingPayload", map[string]interface{}{
		"payload": fmt.Sprintf("%+v", q),
	})
	space := SpaceFrom(q.IAMWorkloadIdentityBinding.Binding.Scope)
	return IAMWorkloadIdentityBindingResourceModel{
		SpaceID:          types.StringValue(space.ID()),
		Mrn:              types.StringValue(q.IAMWorkloadIdentityBinding.Binding.Mrn),
		Name:             types.StringValue(q.IAMWorkloadIdentityBinding.Binding.Name),
		Description:      types.StringValue(q.IAMWorkloadIdentityBinding.Binding.Description),
		IssuerURI:        types.StringValue(q.IAMWorkloadIdentityBinding.Binding.IssuerURI),
		Subject:          types.StringValue(q.IAMWorkloadIdentityBinding.Binding.Subject),
		Expiration:       types.Int32Value(q.IAMWorkloadIdentityBinding.Binding.Expiration),
		Roles:            ConvertListValue(q.IAMWorkloadIdentityBinding.Binding.Roles),
		AllowedAudiences: ConvertListValue(q.IAMWorkloadIdentityBinding.Binding.AllowedAudiences),
		Mappings:         types.MapNull(types.StringType),
	}, nil
}

func (r *IAMWorkloadIdentityBindingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data IAMWorkloadIdentityBindingResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	m, err := r.readIAMWorkloadIdentityBinding(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to read binding. Got error: %s", err),
		)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// Update is not allowed by design. We only read and exist.
func (r *IAMWorkloadIdentityBindingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data IAMWorkloadIdentityBindingResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
}

type DeletePayload struct {
	Mrn mondoov1.String
}

func (r *IAMWorkloadIdentityBindingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data IAMWorkloadIdentityBindingResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to delete the resource.
	var deleteMutation struct {
		RemoveWIFAuthBinding DeletePayload `graphql:"removeWIFAuthBinding(mrn: $mrn)"`
	}

	variables := map[string]interface{}{
		"mrn": mondoov1.String(data.Mrn.ValueString()),
	}
	tflog.Debug(ctx, "RemoveWIFAuthBindingVariables", map[string]interface{}{
		"input": fmt.Sprintf("%+v", variables),
	})
	err := r.client.Mutate(ctx, &deleteMutation, nil, variables)
	if err != nil {
		resp.Diagnostics.AddError(
			"Client Error",
			fmt.Sprintf("Unable to delete binding. Got error: %s", err),
		)
	}
}

func (r *IAMWorkloadIdentityBindingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	mrn := req.ID

	m, err := r.readIAMWorkloadIdentityBinding(ctx, mrn)
	if err != nil {
		resp.
			Diagnostics.
			AddError("Client Error",
				fmt.Sprintf(
					"Unable to import binding. Got error: %s", err,
				),
			)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
