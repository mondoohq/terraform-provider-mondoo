// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"

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
var _ resource.Resource = (*integrationGcpResource)(nil)
var _ resource.ResourceWithImportState = (*integrationGcpResource)(nil)
var _ resource.ResourceWithConfigValidators = (*integrationGcpResource)(nil)

func NewIntegrationGcpResource() resource.Resource {
	return &integrationGcpResource{}
}

type integrationGcpResource struct {
	client *ExtendedGqlClient
}

type integrationGcpResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn        types.String `tfsdk:"mrn"`
	Name       types.String `tfsdk:"name"`
	ProjectId  types.String `tfsdk:"project_id"`
	WifSubject types.String `tfsdk:"wif_subject"`

	// credentials
	Credential integrationGcpCredentialModel `tfsdk:"credentials"`
}

type integrationGcpCredentialModel struct {
	PrivateKey types.String           `tfsdk:"private_key"`
	Wif        *gcpWifCredentialModel `tfsdk:"wif"`
}

type gcpWifCredentialModel struct {
	Audience            types.String `tfsdk:"audience"`
	ServiceAccountEmail types.String `tfsdk:"service_account_email"`
}

func stringOrNull(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func (m integrationGcpResourceModel) GetConfigurationOptions() *mondoov1.GcpConfigurationOptionsInput {
	opts := &mondoov1.GcpConfigurationOptionsInput{
		ProjectId:   mondoov1.NewStringPtr(mondoov1.String(m.ProjectId.ValueString())),
		DiscoverAll: mondoov1.NewBooleanPtr(mondoov1.Boolean(true)),
	}

	if !m.Credential.PrivateKey.IsNull() && !m.Credential.PrivateKey.IsUnknown() {
		opts.ServiceAccount = mondoov1.NewStringPtr(mondoov1.String(m.Credential.PrivateKey.ValueString()))
	}

	if m.Credential.Wif != nil {
		opts.WifAudience = mondoov1.NewStringPtr(mondoov1.String(m.Credential.Wif.Audience.ValueString()))
		if !m.Credential.Wif.ServiceAccountEmail.IsNull() && !m.Credential.Wif.ServiceAccountEmail.IsUnknown() {
			opts.WifServiceAccountEmail = mondoov1.NewStringPtr(mondoov1.String(m.Credential.Wif.ServiceAccountEmail.ValueString()))
		}
	}

	return opts
}

func (r *integrationGcpResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_gcp"
}

func (r *integrationGcpResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan GCP organizations and projects for misconfigurations and vulnerabilities.`,
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
			"project_id": schema.StringAttribute{
				MarkdownDescription: "GCP project ID",
				Optional:            true,
			},
			"wif_subject": schema.StringAttribute{
				MarkdownDescription: "Computed OIDC subject used when Mondoo requests a WIF token for this integration. Configure your cloud provider's trust policy to accept this subject.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"credentials": schema.SingleNestedAttribute{
				MarkdownDescription: "Credentials for the GCP integration. Provide either a static service account `private_key` or a `wif` block for workload identity federation.",
				Required:            true,
				Attributes: map[string]schema.Attribute{
					"private_key": schema.StringAttribute{
						MarkdownDescription: "GCP service account JSON key. Mutually exclusive with `wif`.",
						Optional:            true,
						Sensitive:           true,
						Validators: []validator.String{
							stringvalidator.ConflictsWith(
								path.MatchRoot("credentials").AtName("wif"),
							),
						},
					},
					"wif": schema.SingleNestedAttribute{
						MarkdownDescription: "Workload identity federation configuration. Mutually exclusive with `private_key`.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"audience": schema.StringAttribute{
								MarkdownDescription: "WIF audience URL for GCP workload identity federation.",
								Required:            true,
							},
							"service_account_email": schema.StringAttribute{
								MarkdownDescription: "Optional GCP service account email to impersonate via workload identity federation.",
								Optional:            true,
							},
						},
						Validators: []validator.Object{
							objectvalidator.ConflictsWith(
								path.MatchRoot("credentials").AtName("private_key"),
							),
						},
					},
				},
			},
		},
	}
}

func (r *integrationGcpResource) ConfigValidators(ctx context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.AtLeastOneOf(
			path.MatchRoot("credentials").AtName("private_key"),
			path.MatchRoot("credentials").AtName("wif"),
		),
	}
}

func (r *integrationGcpResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationGcpResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationGcpResourceModel

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
		mondoov1.ClientIntegrationTypeGcp,
		mondoov1.ClientIntegrationConfigurationInput{
			GcpConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create GCP integration. Got error: %s", err),
			)
		return
	}

	// trigger integration to gather results quickly after the first setup
	// NOTE: we ignore the error since the integration state does not depend on it
	_, err = r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunScan)
	if err != nil {
		resp.Diagnostics.
			AddWarning("Client Error",
				fmt.Sprintf("Unable to trigger integration. Got error: %s", err),
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
		data.WifSubject = types.StringValue(fetched.ConfigurationOptions.GcpConfigurationOptions.WifSubject)
	}

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGcpResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationGcpResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Refresh server-computed fields (e.g. wif_subject) from the API.
	integration, err := r.client.GetClientIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Error reading GCP integration", err.Error())
		return
	}
	opts := integration.ConfigurationOptions.GcpConfigurationOptions
	data.WifSubject = types.StringValue(opts.WifSubject)
	if data.Credential.Wif != nil {
		data.Credential.Wif.Audience = types.StringValue(opts.WifAudience)
		data.Credential.Wif.ServiceAccountEmail = stringOrNull(opts.WifServiceAccountEmail)
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGcpResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationGcpResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		GcpConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeGcp,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update GCP integration. Got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGcpResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationGcpResourceModel

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
				fmt.Sprintf("Unable to delete GCP integration. Got error: %s", err),
			)
		return
	}
}

func (r *integrationGcpResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {

	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	opts := integration.ConfigurationOptions.GcpConfigurationOptions
	model := integrationGcpResourceModel{
		Mrn:        types.StringValue(integration.Mrn),
		Name:       types.StringValue(integration.Name),
		SpaceID:    types.StringValue(integration.SpaceID()),
		ProjectId:  types.StringValue(opts.ProjectId),
		WifSubject: types.StringValue(opts.WifSubject),
		Credential: integrationGcpCredentialModel{
			PrivateKey: types.StringPointerValue(nil),
		},
	}
	if opts.WifAudience != "" {
		model.Credential.Wif = &gcpWifCredentialModel{
			Audience:            types.StringValue(opts.WifAudience),
			ServiceAccountEmail: stringOrNull(opts.WifServiceAccountEmail),
		}
	}

	resp.State.Set(ctx, &model)
}
