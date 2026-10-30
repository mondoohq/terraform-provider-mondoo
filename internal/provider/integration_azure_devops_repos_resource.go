// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
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

var _ resource.Resource = (*integrationAzureDevopsReposResource)(nil)
var _ resource.ResourceWithImportState = (*integrationAzureDevopsReposResource)(nil)
var _ resource.ResourceWithConfigValidators = (*integrationAzureDevopsReposResource)(nil)

func NewIntegrationAzureDevopsReposResource() resource.Resource {
	return &integrationAzureDevopsReposResource{}
}

type integrationAzureDevopsReposResource struct {
	client *ExtendedGqlClient
}

type integrationAzureDevopsReposResourceModel struct {
	OrgID   types.String `tfsdk:"org_id"`
	SpaceID types.String `tfsdk:"space_id"`
	Mrn     types.String `tfsdk:"mrn"`
	Name    types.String `tfsdk:"name"`

	Organization types.String `tfsdk:"organization"`
	Project      types.String `tfsdk:"project"`
	Repository   types.String `tfsdk:"repository"`

	RepositoryAllowList types.List `tfsdk:"repository_allow_list"`
	RepositoryDenyList  types.List `tfsdk:"repository_deny_list"`

	DiscoverTerraform    types.Bool `tfsdk:"discover_terraform"`
	DiscoverK8sManifests types.Bool `tfsdk:"discover_k8s_manifests"`
	CodeScan             types.Bool `tfsdk:"code_scan"`
	SecretScan           types.Bool `tfsdk:"secret_scan"`
	VulnerabilityScan    types.Bool `tfsdk:"vulnerability_scan"`

	Credentials *azureDevopsReposCredentialsModel `tfsdk:"credentials"`
}

type azureDevopsReposCredentialsModel struct {
	TenantID     types.String `tfsdk:"tenant_id"`
	ClientID     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Certificate  types.String `tfsdk:"certificate"`
}

func (r *integrationAzureDevopsReposResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_azure_devops_repos"
}

// azureDevopsOrganizationName is the organization in https://dev.azure.com/<name>.
var azureDevopsOrganizationName = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]*[A-Za-z0-9])?$`)

func (r *integrationAzureDevopsReposResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Continuously scan Azure DevOps repositories: every repository of an Azure DevOps organization, " +
			"or one repository when `project` and `repository` are set. " +
			"Create it in a Mondoo organization (`org_id`), whose routing rules place the assets into spaces, or in a space (`space_id`). " +
			"Authenticates as a Microsoft Entra app registration, with a client secret or a certificate.",
		Attributes: map[string]schema.Attribute{
			"org_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo organization identifier. Set this to create the integration in the organization. Conflicts with `space_id`.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("space_id")),
				},
			},
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo space identifier. If neither `space_id` nor `org_id` is set, the provider space is used.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mrn": schema.StringAttribute{
				MarkdownDescription: "Integration identifier.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the integration.",
				Required:            true,
				Validators: []validator.String{
					stringvalidator.LengthBetween(1, 250),
				},
			},
			"organization": schema.StringAttribute{
				MarkdownDescription: "Azure DevOps organization name: the `acme` in `https://dev.azure.com/acme`. Changing it replaces the integration.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.RegexMatches(azureDevopsOrganizationName,
						"must be the Azure DevOps organization name, like acme, not its URL"),
				},
			},
			"project": schema.StringAttribute{
				MarkdownDescription: "Project of the one repository to scan. Set together with `repository`. Leave both unset to scan the whole organization. Changing it replaces the integration.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("repository")),
				},
			},
			"repository": schema.StringAttribute{
				MarkdownDescription: "The one repository to scan, by name. Set together with `project`. Changing it replaces the integration.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.AlsoRequires(path.MatchRoot("project")),
				},
			},
			"repository_allow_list": schema.ListAttribute{
				MarkdownDescription: "Scan only the repositories matching these glob patterns. Whole-organization mode only.",
				Optional:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					listvalidator.ConflictsWith(path.MatchRoot("repository")),
				},
			},
			"repository_deny_list": schema.ListAttribute{
				MarkdownDescription: "Scan all repositories except the ones matching these glob patterns. Whole-organization mode only.",
				Optional:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					listvalidator.ConflictsWith(path.MatchRoot("repository")),
				},
			},
			"discover_terraform":     optionalBool("Discover Terraform files in the repositories.", false),
			"discover_k8s_manifests": optionalBool("Discover Kubernetes manifests in the repositories.", false),
			"code_scan":              optionalBool("Scan the source code (SAST).", false),
			"secret_scan":            optionalBool("Detect secrets in the repositories.", false),
			"vulnerability_scan":     optionalBool("Scan dependencies for vulnerabilities (SCA).", false),
			"credentials": schema.SingleNestedAttribute{
				MarkdownDescription: "Microsoft Entra app registration to authenticate as. Set exactly one of `client_secret` and `certificate`; " +
					"switching between them replaces the integration. Mondoo checks the credentials against Azure DevOps before it saves the integration.",
				Required: true,
				Attributes: map[string]schema.Attribute{
					"tenant_id": schema.StringAttribute{
						MarkdownDescription: "Entra tenant (directory) ID.",
						Required:            true,
					},
					"client_id": schema.StringAttribute{
						MarkdownDescription: "Application (client) ID of the app registration.",
						Required:            true,
					},
					"client_secret": schema.StringAttribute{
						MarkdownDescription: "Client secret of the app registration.",
						Optional:            true,
						Sensitive:           true,
						PlanModifiers: []planmodifier.String{
							replaceOnCredentialSwitch("certificate"),
						},
					},
					"certificate": schema.StringAttribute{
						MarkdownDescription: "PEM bundle with the certificate uploaded to the app registration and its unencrypted RSA private key.",
						Optional:            true,
						Sensitive:           true,
						PlanModifiers: []planmodifier.String{
							replaceOnCredentialSwitch("client_secret"),
						},
					},
				},
			},
		},
	}
}

func (r *integrationAzureDevopsReposResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(
			path.MatchRoot("credentials").AtName("client_secret"),
			path.MatchRoot("credentials").AtName("certificate"),
		),
	}
}

// replaceOnCredentialSwitch replaces the integration when this secret is set and
// state holds the other one: the API fixes the credential mode at create.
func replaceOnCredentialSwitch(other string) planmodifier.String {
	return stringplanmodifier.RequiresReplaceIf(
		func(ctx context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
			if req.PlanValue.IsNull() || req.State.Raw.IsNull() {
				return
			}
			var stored types.String
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("credentials").AtName(other), &stored)...)
			resp.RequiresReplace = !stored.IsNull()
		},
		"Switching between client_secret and certificate replaces the integration.",
		"Switching between `client_secret` and `certificate` replaces the integration.",
	)
}

func (r *integrationAzureDevopsReposResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	client, ok := req.ProviderData.(*ExtendedGqlClient)
	if !ok {
		resp.Diagnostics.AddError("Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ExtendedGqlClient. Got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	r.client = client
}

// configInput builds the API input. withCredential sends the credential arm; an
// update leaves it out when the app registration is unchanged, and the API keeps
// the stored one. The lists and toggles are always sent: the API replaces them
// as a set and treats an unset toggle as off.
func (m integrationAzureDevopsReposResourceModel) configInput(withCredential bool) mondoov1.ClientIntegrationConfigurationInput {
	b := func(v types.Bool) *mondoov1.Boolean { return mondoov1.NewBooleanPtr(mondoov1.Boolean(v.ValueBool())) }
	allow := ConvertSliceStrings(m.RepositoryAllowList)
	deny := ConvertSliceStrings(m.RepositoryDenyList)
	opts := &mondoov1.AzureDevopsReposConfigurationOptionsInput{
		Organization:         mondoov1.NewStringPtr(mondoov1.String(m.Organization.ValueString())),
		Project:              OptionalString(m.Project),
		Repository:           OptionalString(m.Repository),
		ReposAllowList:       &allow,
		ReposDenyList:        &deny,
		DiscoverTerraform:    b(m.DiscoverTerraform),
		DiscoverK8sManifests: b(m.DiscoverK8sManifests),
		EnableCodeScan:       b(m.CodeScan),
		EnableSecretScan:     b(m.SecretScan),
		EnableVulnScan:       b(m.VulnerabilityScan),
	}

	c := m.Credentials
	tenant, client := mondoov1.String(c.TenantID.ValueString()), mondoov1.String(c.ClientID.ValueString())
	if c.Certificate.IsNull() {
		mode := mondoov1.AzureDevopsReposCredentialModeEntraClientSecret
		opts.CredentialMode = &mode
		if withCredential {
			opts.EntraClientSecret = &mondoov1.AzureDevopsReposEntraClientSecretInput{
				TenantId: tenant, ClientId: client, ClientSecret: OptionalString(c.ClientSecret),
			}
		}
	} else {
		mode := mondoov1.AzureDevopsReposCredentialModeEntraCertificate
		opts.CredentialMode = &mode
		if withCredential {
			opts.EntraCertificate = &mondoov1.AzureDevopsReposEntraCertificateInput{
				TenantId: tenant, ClientId: client, Certificate: OptionalString(c.Certificate),
			}
		}
	}
	return mondoov1.ClientIntegrationConfigurationInput{AzureDevopsReposConfigurationOptions: opts}
}

// azureDevopsReposModelFrom builds state from the API. The secret is never
// returned, so it comes from prior state (nil on import). Names the user wrote
// in another case than Azure DevOps reports are kept, so they don't plan a
// replacement.
func azureDevopsReposModelFrom(integration Integration, prior *integrationAzureDevopsReposResourceModel) integrationAzureDevopsReposResourceModel {
	o := integration.ConfigurationOptions.AzureDevopsReposConfigurationOptions
	model := integrationAzureDevopsReposResourceModel{
		OrgID:                types.StringNull(),
		SpaceID:              types.StringNull(),
		Mrn:                  types.StringValue(integration.Mrn),
		Name:                 types.StringValue(integration.Name),
		Organization:         types.StringValue(o.Organization),
		Project:              optionalStringValue(o.Project),
		Repository:           optionalStringValue(o.Repository),
		RepositoryAllowList:  optionalListValue(o.ReposAllowList),
		RepositoryDenyList:   optionalListValue(o.ReposDenyList),
		DiscoverTerraform:    types.BoolValue(o.DiscoverTerraform),
		DiscoverK8sManifests: types.BoolValue(o.DiscoverK8sManifests),
		CodeScan:             types.BoolValue(o.EnableCodeScan),
		SecretScan:           types.BoolValue(o.EnableSecretScan),
		VulnerabilityScan:    types.BoolValue(o.EnableVulnScan),
		Credentials: &azureDevopsReposCredentialsModel{
			ClientSecret: types.StringNull(),
			Certificate:  types.StringNull(),
		},
	}
	if orgID, ok := strings.CutPrefix(integration.ScopeMRN(), orgPrefix); ok {
		model.OrgID = types.StringValue(orgID)
	} else {
		model.SpaceID = types.StringValue(integration.SpaceID())
	}

	identity := o.EntraClientSecret
	if o.CredentialMode == string(mondoov1.AzureDevopsReposCredentialModeEntraCertificate) {
		identity = o.EntraCertificate
	}
	model.Credentials.TenantID = types.StringValue(identity.TenantId)
	model.Credentials.ClientID = types.StringValue(identity.ClientId)

	if prior == nil {
		return model
	}
	keepCase := func(stored, read types.String) types.String {
		if strings.EqualFold(stored.ValueString(), read.ValueString()) {
			return stored
		}
		return read
	}
	model.Organization = keepCase(prior.Organization, model.Organization)
	model.Project = keepCase(prior.Project, model.Project)
	model.Repository = keepCase(prior.Repository, model.Repository)
	if prior.Credentials != nil {
		model.Credentials.ClientSecret = prior.Credentials.ClientSecret
		model.Credentials.Certificate = prior.Credentials.Certificate
	}
	return model
}

func optionalStringValue(s string) types.String {
	if s == "" {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func optionalListValue(list []string) types.List {
	if len(list) == 0 {
		return types.ListNull(types.StringType)
	}
	return ConvertListValue(list)
}

func (r *integrationAzureDevopsReposResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationAzureDevopsReposResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var scopeMrn string
	if orgID := data.OrgID.ValueString(); orgID != "" {
		scopeMrn = orgPrefix + orgID
		data.SpaceID = types.StringNull()
	} else {
		space, err := r.client.ComputeSpace(data.SpaceID)
		if err != nil {
			resp.Diagnostics.AddError("Invalid Configuration", err.Error())
			return
		}
		scopeMrn = space.MRN()
		data.SpaceID = types.StringValue(space.ID())
	}
	ctx = tflog.SetField(ctx, "scope_mrn", scopeMrn)

	tflog.Debug(ctx, "Creating Azure DevOps repository scanning integration")
	integration, err := r.client.CreateScopedIntegration(ctx,
		scopeMrn,
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAzureDevopsRepos,
		data.configInput(true),
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to create Azure DevOps repository scanning integration. Got error: %s", err))
		return
	}

	data.Mrn = types.StringValue(string(integration.Mrn))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAzureDevopsReposResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationAzureDevopsReposResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	integration, err := r.client.GetClientIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to read Azure DevOps repository scanning integration. Got error: %s", err))
		return
	}

	model := azureDevopsReposModelFrom(integration, &data)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *integrationAzureDevopsReposResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state integrationAzureDevopsReposResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Re-send the app registration only when it changed: the API validates it
	// against Azure DevOps again and stores a new credential.
	credentialChanged := state.Credentials == nil || *plan.Credentials != *state.Credentials

	_, err := r.client.UpdateIntegration(ctx,
		plan.Mrn.ValueString(),
		plan.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAzureDevopsRepos,
		plan.configInput(credentialChanged),
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to update Azure DevOps repository scanning integration. Got error: %s", err))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *integrationAzureDevopsReposResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationAzureDevopsReposResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to delete Azure DevOps repository scanning integration. Got error: %s", err))
	}
}

// ImportState imports by integration MRN. The secret is never returned: set
// client_secret or certificate in the configuration, and the next apply sends it.
func (r *integrationAzureDevopsReposResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}
	model := azureDevopsReposModelFrom(*integration, nil)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}
