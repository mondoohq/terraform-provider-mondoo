// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/objectvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
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

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = (*integrationAwsResource)(nil)
var _ resource.ResourceWithImportState = (*integrationAwsResource)(nil)
var _ resource.ResourceWithConfigValidators = (*integrationAwsResource)(nil)

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
	Mrn        types.String `tfsdk:"mrn"`
	Name       types.String `tfsdk:"name"`
	WifSubject types.String `tfsdk:"wif_subject"`

	// AWS credentials
	Credential integrationAwsCredentialModel `tfsdk:"credentials"`
}

type integrationAwsCredentialModel struct {
	Role *roleCredentialModel      `tfsdk:"role"`
	Key  *accessKeyCredentialModel `tfsdk:"key"`
	Wif  *awsWifCredentialModel    `tfsdk:"wif"`
}

type roleCredentialModel struct {
	RoleArn    types.String `tfsdk:"role_arn"`
	ExternalId types.String `tfsdk:"external_id"`
}

type accessKeyCredentialModel struct {
	AccessKey types.String `tfsdk:"access_key"`
	SecretKey types.String `tfsdk:"secret_key"`
}

type awsWifCredentialModel struct {
	Audience types.String `tfsdk:"audience"`
	RoleArn  types.String `tfsdk:"role_arn"`
}

func (m integrationAwsResourceModel) GetConfigurationOptions() *mondoov1.HostedAwsConfigurationOptionsInput {
	opts := &mondoov1.HostedAwsConfigurationOptionsInput{}

	if m.Credential.Key != nil {
		opts.KeyCredential = &mondoov1.AWSSecretKeyCredential{
			AccessKeyId:     mondoov1.String(m.Credential.Key.AccessKey.ValueString()),
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
			ExternalId: externalID,
		}
	}

	if m.Credential.Wif != nil {
		opts.WifCredential = &mondoov1.AWSWifCredential{
			Audience: mondoov1.String(m.Credential.Wif.Audience.ValueString()),
			RoleArn:  mondoov1.String(m.Credential.Wif.RoleArn.ValueString()),
		}
	}

	return opts
}

func (r *integrationAwsResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_aws"
}

func (r *integrationAwsResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan AWS accounts for misconfigurations and vulnerabilities.`,
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
			"wif_subject": schema.StringAttribute{
				MarkdownDescription: "Computed OIDC subject used when Mondoo requests a WIF token for this integration. Configure your cloud provider's trust policy to accept this subject.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"credentials": schema.SingleNestedAttribute{
				MarkdownDescription: "Credentials for the AWS integration. Exactly one of `role`, `key`, or `wif` must be configured.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"role": schema.SingleNestedAttribute{
						MarkdownDescription: "IAM role credentials. Mutually exclusive with `key` and `wif`.",
						Optional:            true,
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
							objectvalidator.ConflictsWith(
								path.MatchRoot("credentials").AtName("key"),
								path.MatchRoot("credentials").AtName("wif"),
							),
						},
					},
					"key": schema.SingleNestedAttribute{
						MarkdownDescription: "Static IAM access key credentials. Mutually exclusive with `role` and `wif`.",
						Optional:            true,
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
						Validators: []validator.Object{
							objectvalidator.ConflictsWith(
								path.MatchRoot("credentials").AtName("role"),
								path.MatchRoot("credentials").AtName("wif"),
							),
						},
					},
					"wif": schema.SingleNestedAttribute{
						MarkdownDescription: "Workload identity federation credentials. Uses Mondoo as an OIDC identity provider to assume an IAM role via web identity. Mutually exclusive with `role` and `key`.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"audience": schema.StringAttribute{
								MarkdownDescription: "Audience value configured in the AWS IAM OIDC identity provider.",
								Required:            true,
							},
							"role_arn": schema.StringAttribute{
								MarkdownDescription: "ARN of the IAM role to assume via web identity federation.",
								Required:            true,
							},
						},
						Validators: []validator.Object{
							objectvalidator.ConflictsWith(
								path.MatchRoot("credentials").AtName("role"),
								path.MatchRoot("credentials").AtName("key"),
							),
						},
					},
				},
			},
		},
	}
}

func (r *integrationAwsResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot("credentials").AtName("role"),
			path.MatchRoot("credentials").AtName("key"),
			path.MatchRoot("credentials").AtName("wif"),
		),
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
			fmt.Sprintf("Expected *http.Client. Got: %T. Please report this issue to the provider developers.", req.ProviderData),
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
				fmt.Sprintf("Unable to create AWS integration. Got error: %s", err),
			)
		return
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))
	data.SpaceID = types.StringValue(space.ID())

	// Fetch the full integration to populate server-computed fields (e.g. wif_subject)
	fetched, err := r.client.GetClientIntegration(ctx, string(integration.Mrn))
	if err != nil {
		resp.Diagnostics.AddWarning("Client Warning",
			fmt.Sprintf("Unable to fetch integration after create to populate computed fields. Got error: %s", err))
		data.WifSubject = types.StringNull()
	} else {
		data.WifSubject = types.StringValue(fetched.ConfigurationOptions.HostedAwsConfigurationOptions.WifSubject)
	}

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

	// Refresh server-computed fields (e.g. wif_subject) from the API.
	integration, err := r.client.GetClientIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading AWS integration", err.Error())
		return
	}
	opts := integration.ConfigurationOptions.HostedAwsConfigurationOptions
	data.WifSubject = types.StringValue(opts.WifSubject)
	if data.Credential.Wif != nil {
		data.Credential.Wif.Audience = types.StringValue(opts.WifAudience)
		data.Credential.Wif.RoleArn = types.StringValue(opts.WifRoleArn)
	}

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
				fmt.Sprintf("Unable to update AWS integration. Got error: %s", err),
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
				fmt.Sprintf("Unable to delete AWS integration. Got error: %s", err),
			)
		return
	}
}

func (r *integrationAwsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	opts := integration.ConfigurationOptions.HostedAwsConfigurationOptions
	model := integrationAwsResourceModel{
		SpaceID:    types.StringValue(integration.SpaceID()),
		Mrn:        types.StringValue(integration.Mrn),
		Name:       types.StringValue(integration.Name),
		WifSubject: types.StringValue(opts.WifSubject),
	}

	switch {
	case opts.WifAudience != "" || opts.WifRoleArn != "":
		model.Credential.Wif = &awsWifCredentialModel{
			Audience: types.StringValue(opts.WifAudience),
			RoleArn:  types.StringValue(opts.WifRoleArn),
		}
	case opts.AccessKeyId != "":
		model.Credential.Key = &accessKeyCredentialModel{
			AccessKey: types.StringValue(opts.AccessKeyId),
			SecretKey: types.StringPointerValue(nil), // cannot be imported
		}
	case opts.Role != "":
		model.Credential.Role = &roleCredentialModel{
			RoleArn:    types.StringValue(opts.Role),
			ExternalId: types.StringPointerValue(nil), // cannot be imported
		}
	}

	resp.State.Set(ctx, &model)
}
