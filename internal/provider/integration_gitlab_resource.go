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

var _ resource.Resource = (*integrationGitlabResource)(nil)

func NewIntegrationGitlabResource() resource.Resource {
	return &integrationGitlabResource{}
}

type integrationGitlabResource struct {
	client *ExtendedGqlClient
}

type integrationGitlabResourceModel struct {
	SpaceID types.String `tfsdk:"space_id"`

	// Integration details
	Mrn  types.String `tfsdk:"mrn"`
	Name types.String `tfsdk:"name"`
	// Configuration options
	Group     types.String                     `tfsdk:"group"`
	BaseURL   types.String                     `tfsdk:"base_url"`
	Discovery *integrationGitlabDiscoveryModel `tfsdk:"discovery"`
	// credentials
	Credential *integrationGitlabCredentialModel `tfsdk:"credentials"`
}

type integrationGitlabDiscoveryModel struct {
	Groups       types.Bool `tfsdk:"groups"`
	Projects     types.Bool `tfsdk:"projects"`
	Terraform    types.Bool `tfsdk:"terraform"`
	K8sManifests types.Bool `tfsdk:"k8s_manifests"`
}

type integrationGitlabCredentialModel struct {
	Token types.String `tfsdk:"token"`
}

func (r *integrationGitlabResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_gitlab"
}

func (r *integrationGitlabResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan GitLab for misconfigurations.`,
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
			"group": schema.StringAttribute{
				MarkdownDescription: "Group to assign the integration to.",
				Optional:            true,
			},
			"base_url": schema.StringAttribute{
				MarkdownDescription: "Base URL of the GitLab instance (only set this if your instance is self-hosted).",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.RegexMatches(
						regexp.MustCompile(`^https?:\/\/[a-zA-Z0-9\-._~:\/?#[\]@!$&'()*+,;=%]+$`),
						"must be a valid URL",
					),
				},
			},
			"discovery": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
					"groups": schema.BoolAttribute{
						MarkdownDescription: "Enable discovery of GitLab groups.",
						Optional:            true,
					},
					"projects": schema.BoolAttribute{
						MarkdownDescription: "Enable discovery of GitLab projects.",
						Optional:            true,
					},
					"terraform": schema.BoolAttribute{
						MarkdownDescription: "Enable discovery of Terraform configurations.",
						Optional:            true,
					},
					"k8s_manifests": schema.BoolAttribute{
						MarkdownDescription: "Enable discovery of Kubernetes manifests.",
						Optional:            true,
					},
				},
			},
			"credentials": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"token": schema.StringAttribute{
						MarkdownDescription: "Token for GitLab integration.",
						Required:            true,
						Sensitive:           true,
					},
				},
			},
		},
	}
}

func (r *integrationGitlabResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationGitlabResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationGitlabResourceModel

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

	gitlabType := mondoov1.GitlabIntegrationType("NONE")
	if data.Group.ValueString() != "" {
		gitlabType = mondoov1.GitlabIntegrationType("GROUP")
	}
	// Create API call logic
	tflog.Debug(ctx, "Creating integration")
	integration, err := r.client.CreateIntegration(ctx,
		space.MRN(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeGitLab,
		mondoov1.ClientIntegrationConfigurationInput{
			GitLabConfigurationOptions: &mondoov1.GitlabConfigurationOptionsInput{
				Type:                 gitlabType,
				Group:                mondoov1.NewStringPtr(mondoov1.String(data.Group.ValueString())),
				BaseURL:              mondoov1.NewStringPtr(mondoov1.String(data.BaseURL.ValueString())),
				DiscoverGroups:       mondoov1.NewBooleanPtr(mondoov1.Boolean(data.Discovery.Groups.ValueBool())),
				DiscoverProjects:     mondoov1.NewBooleanPtr(mondoov1.Boolean(data.Discovery.Projects.ValueBool())),
				DiscoverTerraform:    mondoov1.NewBooleanPtr(mondoov1.Boolean(data.Discovery.Terraform.ValueBool())),
				DiscoverK8sManifests: mondoov1.NewBooleanPtr(mondoov1.Boolean(data.Discovery.K8sManifests.ValueBool())),
				Token:                mondoov1.NewStringPtr(mondoov1.String(data.Credential.Token.ValueString())),
			},
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create GitLab integration, got error: %s", err),
			)
		return
	}

	// trigger integration to gather results quickly after the first setup
	// NOTE: we ignore the error since the integration state does not depend on it
	_, err = r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunScan)
	if err != nil {
		resp.Diagnostics.
			AddWarning("Client Error",
				fmt.Sprintf("Unable to trigger integration, got error: %s", err),
			)
		return
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(data.Name.ValueString())
	data.SpaceID = types.StringValue(space.ID())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGitlabResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationGitlabResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGitlabResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationGitlabResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	gitlabType := mondoov1.GitlabIntegrationType("NONE")
	if data.Group.ValueString() != "" {
		gitlabType = mondoov1.GitlabIntegrationType("GROUP")
	}

	opts := mondoov1.ClientIntegrationConfigurationInput{
		GitLabConfigurationOptions: &mondoov1.GitlabConfigurationOptionsInput{
			Type:                 gitlabType,
			Group:                mondoov1.NewStringPtr(mondoov1.String(data.Group.ValueString())),
			BaseURL:              mondoov1.NewStringPtr(mondoov1.String(data.BaseURL.ValueString())),
			DiscoverGroups:       mondoov1.NewBooleanPtr(mondoov1.Boolean(data.Discovery.Groups.ValueBool())),
			DiscoverProjects:     mondoov1.NewBooleanPtr(mondoov1.Boolean(data.Discovery.Projects.ValueBool())),
			DiscoverTerraform:    mondoov1.NewBooleanPtr(mondoov1.Boolean(data.Discovery.Terraform.ValueBool())),
			DiscoverK8sManifests: mondoov1.NewBooleanPtr(mondoov1.Boolean(data.Discovery.K8sManifests.ValueBool())),
			Token:                mondoov1.NewStringPtr(mondoov1.String(data.Credential.Token.ValueString())),
		},
	}
	// Update API call logic
	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeGitLab,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update GitLab integration, got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGitlabResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationGitlabResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete API call logic
	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to delete GitLab integration, got error: %s", err),
			)
		return
	}
}
