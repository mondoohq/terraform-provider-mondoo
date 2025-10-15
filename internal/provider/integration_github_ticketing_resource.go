// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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
var _ resource.Resource = (*integrationGithubTicketingResource)(nil)
var _ resource.ResourceWithImportState = (*integrationGithubTicketingResource)(nil)

func NewIntegrationGithubTicketingResource() resource.Resource {
	return &integrationGithubTicketingResource{}
}

type integrationGithubTicketingResource struct {
	client *ExtendedGqlClient
}

type integrationGithubTicketingResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn  types.String `tfsdk:"mrn"`
	Name types.String `tfsdk:"name"`

	// GitHub owner and repo for issue tracking
	Owner      types.String `tfsdk:"owner"`
	Repository types.String `tfsdk:"repository"`

	// (Optional.)
	AutoClose  types.Bool `tfsdk:"auto_close"`
	AutoCreate types.Bool `tfsdk:"auto_create"`

	// credentials
	Credential *integrationGithubTicketingCredentialModel `tfsdk:"credentials"`
}

type integrationGithubTicketingCredentialModel struct {
	Token types.String `tfsdk:"token"`
}

func (m integrationGithubTicketingResourceModel) GetConfigurationOptions() *mondoov1.GithubConfigurationOptionsInput {
	opts := &mondoov1.GithubConfigurationOptionsInput{
		Owner:      mondoov1.NewStringPtr(mondoov1.String(m.Owner.ValueString())),
		Repository: mondoov1.NewStringPtr(mondoov1.String(m.Repository.ValueString())),
		Type:       mondoov1.GithubIntegrationTypeRepo,
	}

	if token := m.Credential.Token.ValueString(); token != "" {
		opts.Token = mondoov1.NewStringPtr(mondoov1.String(token))
	}

	return opts
}

func (r *integrationGithubTicketingResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_github_ticketing"
}

func (r *integrationGithubTicketingResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `GitHub ticketing integration to automatically create and manage GitHub issues based on security findings.`,
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo space identifier. If there is no space ID, the provider space is used.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"mrn": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Integration identifier",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the integration.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(250),
				},
			},
			"owner": schema.StringAttribute{
				MarkdownDescription: "GitHub repository owner or organization.",
				Required:            true,
			},
			"repository": schema.StringAttribute{
				MarkdownDescription: "GitHub repository name where issues will be created.",
				Required:            true,
			},
			"auto_close": schema.BoolAttribute{
				MarkdownDescription: "Automatically close issues when the security finding is resolved.",
				Optional:            true,
			},
			"auto_create": schema.BoolAttribute{
				MarkdownDescription: "Automatically create issues for new security findings.",
				Optional:            true,
			},
			"credentials": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"token": schema.StringAttribute{
						MarkdownDescription: "Personal access token for GitHub API access. Must have issue management permissions.",
						Required:            true,
						Sensitive:           true,
						Validators: []validator.String{
							stringvalidator.RegexMatches(
								regexp.MustCompile(`^(ghp_[a-zA-Z0-9]{36}|github_pat_[a-zA-Z0-9]{22}_[a-zA-Z0-9]{59})$`),
								"must be a valid classic GitHub token with 40 characters in length, with a prefix of ghp_ or a fine-grained GitHub token with 93 characters in length, with a prefix of github_pat_",
							),
						},
					},
				},
			},
		},
	}
}

func (r *integrationGithubTicketingResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationGithubTicketingResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationGithubTicketingResourceModel

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
	tflog.Debug(ctx, "Creating integration")
	integration, err := r.client.CreateIntegration(ctx,
		space.MRN(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeGithub,
		mondoov1.ClientIntegrationConfigurationInput{
			GithubConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create GitHub ticketing integration. Got error: %s", err),
			)
		return
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))
	data.SpaceID = types.StringValue(space.ID())
	data.Owner = types.StringValue(data.Owner.ValueString())
	data.Repository = types.StringValue(data.Repository.ValueString())
	data.AutoClose = types.BoolValue(data.AutoClose.ValueBool())
	data.AutoCreate = types.BoolValue(data.AutoCreate.ValueBool())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGithubTicketingResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationGithubTicketingResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGithubTicketingResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationGithubTicketingResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		GithubConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeGithub,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update GitHub ticketing integration. Got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGithubTicketingResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationGithubTicketingResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to delete GitHub ticketing integration. Got error: %s", err),
			)
		return
	}
}

func (r *integrationGithubTicketingResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	opts := integration.ConfigurationOptions.GithubConfigurationOptions
	model := integrationGithubTicketingResourceModel{
		Mrn:        types.StringValue(integration.Mrn),
		Name:       types.StringValue(integration.Name),
		SpaceID:    types.StringValue(integration.SpaceID()),
		Owner:      types.StringValue(opts.Owner),
		Repository: types.StringValue(opts.Repository),
		AutoClose:  types.BoolValue(false),
		AutoCreate: types.BoolValue(false),
		Credential: &integrationGithubTicketingCredentialModel{
			Token: types.StringPointerValue(nil),
		},
	}

	resp.State.Set(ctx, &model)
}
