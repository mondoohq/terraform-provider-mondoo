// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
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

var _ resource.Resource = (*integrationAwsResource)(nil)

func NewIntegrationAwsResource() resource.Resource {
	return &integrationAwsResource{}
}

type integrationAwsResource struct {
	client *ExtendedGqlClient
}

type integrationAwsResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn  types.String `tfsdk:"mrn"`
	Name types.String `tfsdk:"name"`

	// AWS credentials
	Credential integrationAwsCredentialModel `tfsdk:"credentials"`
}

type integrationAwsCredentialModel struct {
	Role *roleCredentialModel      `tfsdk:"role"`
	Key  *accessKeyCredentialModel `tfsdk:"key"`
}

type roleCredentialModel struct {
	RoleArn    types.String `tfsdk:"role_arn"`
	ExternalId types.String `tfsdk:"external_id"`
}

type accessKeyCredentialModel struct {
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
}

func (m integrationAwsResourceModel) GetConfigurationOptions() *mondoov1.HostedAwsConfigurationOptionsInput {
	opts := &mondoov1.HostedAwsConfigurationOptionsInput{}

	if m.Credential.Key != nil {
		opts.KeyCredential = &mondoov1.AWSSecretKeyCredential{
			AccessKeyID:     mondoov1.String(m.Credential.Key.AccessKey.ValueString()),
			SecretAccessKey: mondoov1.String(m.Credential.Key.SecretKey.ValueString()),
		}
	}

	if m.Credential.Role != nil {
		var externalID *mondoov1.String
		externalIDValue := m.Credential.Role.ExternalId.ValueString()
		if externalIDValue == "" {
			externalID = mondoov1.NewStringPtr(mondoov1.String(externalIDValue))
		}

		opts.RoleCredential = &mondoov1.AWSRoleCredential{
			Role:       mondoov1.String(m.Credential.Role.RoleArn.ValueString()),
			ExternalID: externalID,
		}
	}

	return opts
}

func (r *integrationAwsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_aws"
}

func (r *integrationAwsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan AWS organization and accounts for misconfigurations and vulnerabilities.`,
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo Space Identifier. If it is not provided, the provider space is used.",
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
			"credentials": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"role": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"role_arn": schema.StringAttribute{
								Required:  true,
								Sensitive: true,
							},
							"external_id": schema.StringAttribute{
								Optional:  true,
								Sensitive: true,
							},
						},
						Validators: []validator.Object{
							// Validate this attribute must not be configured with other_attr.
							objectvalidator.ConflictsWith(path.Expressions{
								path.MatchRoot("credentials").AtName("key"),
							}...),
						},
					},
					"key": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"access_key": schema.StringAttribute{
								Required:  true,
								Sensitive: true,
								Validators: []validator.String{
									stringvalidator.RegexMatches(
										regexp.MustCompile(`^([A-Z0-9]{20})$`),
										"must be a 20 character string with uppercase letters and numbers only",
									),
								},
							},
							"secret_key": schema.StringAttribute{
								Required:  true,
								Sensitive: true,
								Validators: []validator.String{
									stringvalidator.RegexMatches(
										regexp.MustCompile(`^([a-zA-Z0-9+/]{40})$`),
										"must be a 40 character string with alphanumeric values and + and / only",
									),
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *integrationAwsResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationAwsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationAwsResourceModel

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
		mondoov1.ClientIntegrationTypeAwsHosted,
		mondoov1.ClientIntegrationConfigurationInput{
			AwsHostedConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create AWS integration, got error: %s", err),
			)
		return
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))
	data.SpaceID = types.StringValue(space.ID())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAwsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationAwsResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAwsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationAwsResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		AwsHostedConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAwsHosted,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update AWS integration, got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAwsResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationAwsResourceModel

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
				fmt.Sprintf("Unable to delete AWS integration, got error: %s", err),
			)
		return
	}
}

func (r *integrationAwsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	model := integrationAwsResourceModel{
		SpaceID: types.StringValue(integration.SpaceID()),
		Mrn:     types.StringValue(integration.Mrn),
		Name:    types.StringValue(integration.Name),
		Credential: integrationAwsCredentialModel{
			Role: &roleCredentialModel{
				RoleArn:    types.StringValue(integration.ConfigurationOptions.HostedAwsConfigurationOptions.Role),
				ExternalId: types.StringPointerValue(nil), // cannot be imported
			},
			Key: &accessKeyCredentialModel{
				AccessKey: types.StringValue(integration.ConfigurationOptions.HostedAwsConfigurationOptions.AccessKeyId),
				SecretKey: types.StringPointerValue(nil), // cannot be imported
			},
		},
	}

	resp.State.Set(ctx, &model)
}
