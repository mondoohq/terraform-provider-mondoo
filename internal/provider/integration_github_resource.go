package provider

import (
	"context"
	"fmt"
	"strings"
  "regexp"
  
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var _ resource.Resource = (*integrationGithubResource)(nil)

func NewIntegrationGithubResource() resource.Resource {
	return &integrationGithubResource{}
}

type integrationGithubResource struct {
	client *ExtendedGqlClient
}

type integrationGithubResourceModel struct {
	// scope
	SpaceId types.String `tfsdk:"space_id"`

	// integration details
	Mrn  types.String `tfsdk:"mrn"`
	Name types.String `tfsdk:"name"`

	Owner      types.String `tfsdk:"owner"`
	Repository types.String `tfsdk:"repository"`

	RepositoryAllowList types.List `tfsdk:"repository_allow_list"`
	RepositoryDenyList  types.List `tfsdk:"repository_deny_list"`

	// credentials
	Credential *integrationGithubCredentialModel `tfsdk:"credentials"`
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
				MarkdownDescription: "Mondoo Space Identifier.",
				Required:            true,
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
				MarkdownDescription: "GitHub Owner.",
				Required:            true,
			},
			"repository": schema.StringAttribute{
				MarkdownDescription: "GitHub Repository.",
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
								"must be a valid classic GitHub Token with 40 characters in length, with a prefix of ghp_ or a fine-grained GitHub token with 93 characters in length, with a prefix of github_pat_",
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

	client, ok := req.ProviderData.(*mondoov1.Client)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *http.Client, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = &ExtendedGqlClient{client}
}

func (r *integrationGithubResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {

	var data integrationGithubResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to create the resource.
	spaceMrn := ""
	if data.SpaceId.ValueString() != "" {
		spaceMrn = spacePrefix + data.SpaceId.ValueString()
	}

	integration, err := r.client.CreateIntegration(ctx,
		spaceMrn,
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeGitHub,
		mondoov1.ClientIntegrationConfigurationInput{
			GitHubConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create Domain integration, got error: %s", err))
		return
	}

	// trigger integration to gather results quickly after the first setup
	// NOTE: we ignore the error since the integration state does not depend on it
	_, err = r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunScan)
	if err != nil {
		resp.Diagnostics.AddWarning("Client Error", fmt.Sprintf("Unable to trigger integration, got error: %s", err))
		return
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(data.Name.ValueString())
	data.SpaceId = types.StringValue(data.SpaceId.ValueString())

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

	_, err := r.client.UpdateIntegration(ctx, data.Mrn.ValueString(), data.Name.ValueString(), mondoov1.ClientIntegrationTypeGitHub, opts)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update Domain integration, got error: %s", err))
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
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete Domain integration, got error: %s", err))
		return
	}
}

func (r *integrationGithubResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	mrn := req.ID
	integration, err := r.client.GetClientIntegration(ctx, mrn)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to get GitHub integration, got error: %s", err))
		return
	}

	allowList := r.ConvertListValue(ctx, integration.ConfigurationOptions.GithubConfigurationOptions.ReposAllowList)
	denyList := r.ConvertListValue(ctx, integration.ConfigurationOptions.GithubConfigurationOptions.ReposDenyList)

	model := integrationGithubResourceModel{
		Mrn:                 types.StringValue(mrn),
		Name:                types.StringValue(integration.Name),
		SpaceId:             types.StringValue(strings.Split(integration.Mrn, "/")[len(strings.Split(integration.Mrn, "/"))-3]),
		Owner:               types.StringValue(integration.ConfigurationOptions.GithubConfigurationOptions.Owner),
		Repository:          types.StringValue(integration.ConfigurationOptions.GithubConfigurationOptions.Repository),
		RepositoryAllowList: allowList,
		RepositoryDenyList:  denyList,
		Credential: &integrationGithubCredentialModel{
			Token: types.StringPointerValue(nil),
		},
	}

	if model.Owner.ValueString() == "" {
		model.Owner = types.StringValue(integration.ConfigurationOptions.GithubConfigurationOptions.Organization)
	}

	resp.State.SetAttribute(ctx, path.Root("mrn"), model.Mrn)
	resp.State.SetAttribute(ctx, path.Root("name"), model.Name)
	resp.State.SetAttribute(ctx, path.Root("space_id"), model.SpaceId)
	resp.State.SetAttribute(ctx, path.Root("owner"), model.Owner)
	resp.State.SetAttribute(ctx, path.Root("repository"), model.Repository)
	resp.State.SetAttribute(ctx, path.Root("repository_allow_list"), model.RepositoryAllowList)
	resp.State.SetAttribute(ctx, path.Root("repository_deny_list"), model.RepositoryDenyList)
	resp.State.SetAttribute(ctx, path.Root("credential"), model.Credential)
}

func (r *integrationGithubResource) ConvertListValue(ctx context.Context, list []string) types.List {
	var valueList []attr.Value
	for _, str := range list {
		valueList = append(valueList, types.StringValue(str))
	}
	// Ensure the list is of type types.StringType
	return types.ListValueMust(types.StringType, valueList)
}
