package provider

import (
	"context"
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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
var _ resource.Resource = (*integrationGithubResource)(nil)
var _ resource.ResourceWithImportState = (*integrationGithubResource)(nil)

func NewIntegrationGithubResource() resource.Resource {
	return &integrationGithubResource{}
}

type integrationGithubResource struct {
	client *ExtendedGqlClient
}

type integrationGithubResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn  types.String `tfsdk:"mrn"`
	Name types.String `tfsdk:"name"`

	Owner      types.String `tfsdk:"owner"`
	Repository types.String `tfsdk:"repository"`

	RepositoryAllowList types.List `tfsdk:"repository_allow_list"`
	RepositoryDenyList  types.List `tfsdk:"repository_deny_list"`

	Discovery *integrationGithubDiscoveryModel `tfsdk:"discovery"`

	// credentials
	Credential *integrationGithubCredentialModel `tfsdk:"credentials"`
}

type integrationGithubDiscoveryModel struct {
	// Discover Terraform files in the repositories. (Optional.)
	Terraform types.Bool `tfsdk:"terraform"`
	// Discover k8s manifests in the repositories. (Optional.)
	K8sManifests types.Bool `tfsdk:"k8s_manifests"`
}

type integrationGithubCredentialModel struct {
	Token types.String `tfsdk:"token"`
}

func (m integrationGithubResourceModel) GetConfigurationOptions() *mondoov1.GithubConfigurationOptionsInput {
	opts := &mondoov1.GithubConfigurationOptionsInput{
		Owner:        mondoov1.NewStringPtr(mondoov1.String(m.Owner.ValueString())),
		Organization: mondoov1.NewStringPtr(mondoov1.String(m.Owner.ValueString())),
	}

	repository := m.Repository.ValueString()
	if repository != "" {
		opts.Type = mondoov1.GithubIntegrationTypeRepo
		opts.Repository = mondoov1.NewStringPtr(mondoov1.String(repository))
	} else {
		opts.Type = mondoov1.GithubIntegrationTypeOrg
	}

	token := m.Credential.Token.ValueString()
	if token != "" {
		opts.Token = mondoov1.NewStringPtr(mondoov1.String(token))
	}

	if m.Discovery != nil {
		opts.DiscoverTerraform = mondoov1.NewBooleanPtr(mondoov1.Boolean(m.Discovery.Terraform.ValueBool()))
		opts.DiscoverK8sManifests = mondoov1.NewBooleanPtr(mondoov1.Boolean(m.Discovery.K8sManifests.ValueBool()))
	}

	ctx := context.Background()
	var listAllow []mondoov1.String
	allowlist, _ := m.RepositoryAllowList.ToListValue(ctx)
	allowlist.ElementsAs(ctx, &listAllow, true)

	var listDeny []mondoov1.String
	denylist, _ := m.RepositoryDenyList.ToListValue(ctx)
	denylist.ElementsAs(ctx, &listDeny, true)

	opts.ReposAllowList = &listAllow
	opts.ReposDenyList = &listDeny

	return opts
}

func (r *integrationGithubResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_github"
}

func (r *integrationGithubResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan GitHub organizations and repositories for misconfigurations.`,
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
				MarkdownDescription: "GitHub owner.",
				Required:            true,
			},
			"repository": schema.StringAttribute{
				MarkdownDescription: "GitHub repository.",
				Optional:            true,
			},
			"repository_allow_list": schema.ListAttribute{
				MarkdownDescription: "List of GitHub repositories to scan.",
				Optional:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					// Validate only this attribute or other_attr is configured.
					listvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot("repository_deny_list"),
					}...),
				},
			},
			"repository_deny_list": schema.ListAttribute{
				MarkdownDescription: "List of GitHub repositories to exclude from scanning.",
				Optional:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					// Validate only this attribute or other_attr is configured.
					listvalidator.ConflictsWith(path.Expressions{
						path.MatchRoot("repository_allow_list"),
					}...),
				},
			},
			"discovery": schema.SingleNestedAttribute{
				Optional: true,
				Attributes: map[string]schema.Attribute{
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
						MarkdownDescription: "Token for GitHub integration.",
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

func (r *integrationGithubResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationGithubResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

	var data integrationGithubResourceModel

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
		mondoov1.ClientIntegrationTypeGitHub,
		mondoov1.ClientIntegrationConfigurationInput{
			GitHubConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create GitHub integration. Got error: %s", err),
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
	data.Name = types.StringValue(data.Name.ValueString())
	data.SpaceID = types.StringValue(space.ID())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGithubResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationGithubResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGithubResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationGithubResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		GitHubConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeGitHub,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update GitHub integration. Got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGithubResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationGithubResourceModel

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
				fmt.Sprintf("Unable to delete GitHub integration. Got error: %s", err),
			)
		return
	}
}

func (r *integrationGithubResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	allowList := ConvertListValue(integration.ConfigurationOptions.GithubConfigurationOptions.ReposAllowList)
	denyList := ConvertListValue(integration.ConfigurationOptions.GithubConfigurationOptions.ReposDenyList)

	model := integrationGithubResourceModel{
		Mrn:                 types.StringValue(integration.Mrn),
		Name:                types.StringValue(integration.Name),
		SpaceID:             types.StringValue(integration.SpaceID()),
		Owner:               types.StringValue(integration.ConfigurationOptions.GithubConfigurationOptions.Owner),
		Repository:          types.StringValue(integration.ConfigurationOptions.GithubConfigurationOptions.Repository),
		RepositoryAllowList: allowList,
		RepositoryDenyList:  denyList,
		Discovery: &integrationGithubDiscoveryModel{
			Terraform:    types.BoolValue(integration.ConfigurationOptions.GitlabConfigurationOptions.DiscoverTerraform),
			K8sManifests: types.BoolValue(integration.ConfigurationOptions.GitlabConfigurationOptions.DiscoverK8sManifests),
		},
		Credential: &integrationGithubCredentialModel{
			Token: types.StringPointerValue(nil),
		},
	}

	if model.Owner.ValueString() == "" {
		model.Owner = types.StringValue(integration.ConfigurationOptions.GithubConfigurationOptions.Organization)
	}

	resp.State.Set(ctx, &model)
}
