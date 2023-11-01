// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &RegistrationTokenResource{}
var _ resource.ResourceWithImportState = &RegistrationTokenResource{}

func NewRegistrationTokenResource() resource.Resource {
	return &RegistrationTokenResource{}
}

// RegistrationTokenResource defines the resource implementation.
type RegistrationTokenResource struct {
	client *mondoov1.Client
}

// RegistrationTokenResourceModel describes the resource data model.
type RegistrationTokenResourceModel struct {
	Mrn types.String `tfsdk:"mrn"`

	// scope
	SpaceId types.String `tfsdk:"space_id"`

	// registration token details
	Description  types.String `tfsdk:"description"`
	NoExpiration types.Bool   `tfsdk:"no_exipration"`
	ExpiresIn    types.String `tfsdk:"expires_in"`

	// output
	ExpiresAt types.String `tfsdk:"expires_at"`
	Revoked   types.Bool   `tfsdk:"revoked"`
	Result    types.String `tfsdk:"result"`
}

func (r *RegistrationTokenResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registration_token"
}

func (r *RegistrationTokenResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		// This description is used by the documentation generator and the language server.
		MarkdownDescription: "Example resource",

		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{ // TODO: add check that either space or org needs to be set
				MarkdownDescription: "Example configurable attribute with default value",
				Required:            true,
			},
			"mrn": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Example identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"description": schema.StringAttribute{
				MarkdownDescription: "Example configurable attribute with default value",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"no_exipration": schema.BoolAttribute{ // TODO: add check that either no_exipration or expires_in needs to be set
				MarkdownDescription: "Example configurable attribute with default value",
				Optional:            true,
			},
			"expires_in": schema.StringAttribute{
				MarkdownDescription: "Example configurable attribute with default value",
				Optional:            true,
			},
			"revoked": schema.BoolAttribute{
				MarkdownDescription: "Example configurable attribute with default value",
				Optional:            true,
				Computed:            true,
			},
			"expires_at": schema.StringAttribute{
				MarkdownDescription: "Example configurable attribute with default value",
				Optional:            true,
				Computed:            true,
			},
			"result": schema.StringAttribute{
				Description: "The generated random string.",
				Computed:    true,
				Sensitive:   true,
			},
		},
	}
}

func (r *RegistrationTokenResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*mondoov1.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *RegistrationTokenResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RegistrationTokenResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource
	description := data.Description.ValueString()

	scopeMrn := ""
	if data.SpaceId.ValueString() != "" {
		scopeMrn = spacePrefix + data.SpaceId.ValueString()
	} else {
		resp.Diagnostics.AddError(
			"Either space_id needs to be set",
			"Either space_id needs to be set",
		)
		return
	}

	var expires_in *mondoov1.Int
	if !data.ExpiresIn.IsNull() {
		duration, err := time.ParseDuration(data.ExpiresIn.ValueString())
		if err != nil {
			resp.Diagnostics.AddError(
				"Invalide expires_in value: "+data.ExpiresIn.ValueString(),
				"Invalide expires_in value: "+data.ExpiresIn.ValueString(),
			)
			return
		}
		expires_in = mondoov1.NewIntPtr(mondoov1.Int(duration.Seconds()))
	}

	var noExpiration *mondoov1.Boolean
	if !data.NoExpiration.IsNull() {
		noExpiration = mondoov1.NewBooleanPtr(mondoov1.Boolean(data.NoExpiration.ValueBool()))
	}

	if expires_in != nil && noExpiration != nil {
		resp.Diagnostics.AddError(
			"Either expires_in or no_expiration needs to be set",
			"Either expires_in or no_expiration needs to be set",
		)
		return
	}

	registrationTokenInput := mondoov1.RegistrationTokenInput{
		Description:  mondoov1.NewStringPtr(mondoov1.String(description)),
		ScopeMrn:     mondoov1.NewStringPtr(mondoov1.String(scopeMrn)),
		ExpiresIn:    expires_in,
		NoExpiration: noExpiration,
	}

	tflog.Trace(ctx, "RegistrationTokenInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", registrationTokenInput),
	})

	var generateRegistrationToken struct {
		RegistrationToken struct {
			Mrn         mondoov1.String
			Description mondoov1.String
			Token       mondoov1.String
			Revoked     mondoov1.Boolean
			ExpiresAt   mondoov1.String
		} `graphql:"generateRegistrationToken(input: $input)"`
	}

	err := r.client.Mutate(context.Background(), &generateRegistrationToken, registrationTokenInput, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create space, got error: %s", err))
		return
	}

	// Save space mrn into the Terraform state.
	data.Description = types.StringValue(description)
	data.Mrn = types.StringValue(string(generateRegistrationToken.RegistrationToken.Mrn))
	data.Result = types.StringValue(string(generateRegistrationToken.RegistrationToken.Token))
	data.Revoked = types.BoolValue(bool(generateRegistrationToken.RegistrationToken.Revoked))
	data.ExpiresAt = types.StringValue(string(generateRegistrationToken.RegistrationToken.ExpiresAt))

	// Write logs using the tflog package
	tflog.Trace(ctx, "created a service account resource")

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RegistrationTokenResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RegistrationTokenResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// If applicable, this is a great opportunity to initialize any necessary
	// provider client data and make a call using it.
	// httpResp, err := r.client.Do(httpReq)
	// if err != nil {
	//     resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read example, got error: %s", err))
	//     return
	// }

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RegistrationTokenResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data RegistrationTokenResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *RegistrationTokenResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RegistrationTokenResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to revoke the token.
	var revokeMutation struct {
		RevokeRegistrationTokenResponse struct {
			RevokeRegistrationTokenSuccess struct {
				Ok mondoov1.Boolean
			} `graphql:"... on RevokeRegistrationTokenSuccess"`
			RevokeRegistrationTokenFailure struct {
				Message mondoov1.String
				Code    mondoov1.String
			} `graphql:"... on RevokeRegistrationTokenFailure"`
		} `graphql:"revokeRegistrationToken(input: $input)"`
	}
	revokeInput := mondoov1.RevokeRegistrationTokenInput{
		Mrn: mondoov1.String(spacePrefix + data.SpaceId.ValueString()),
	}
	tflog.Trace(ctx, "RevokeRegistrationTokenInput", map[string]interface{}{
		"input": fmt.Sprintf("%+v", revokeInput),
	})
	err := r.client.Mutate(context.Background(), &revokeMutation, revokeInput, nil)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update service account, got error: %s", err))
		return
	}
}

func (r *RegistrationTokenResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
