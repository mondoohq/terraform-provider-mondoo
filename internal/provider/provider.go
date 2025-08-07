// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	cnquery_config "go.mondoo.com/cnquery/v11/cli/config"
	cnquery_upstream "go.mondoo.com/cnquery/v11/providers-sdk/v1/upstream"
	mondoov1 "go.mondoo.com/mondoo-go"
	"go.mondoo.com/mondoo-go/option"
)

// Ensure MondooProvider satisfies various provider interfaces.
var _ provider.Provider = &MondooProvider{}

// MondooProvider defines the provider implementation.
type MondooProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

// MondooProviderModel describes the provider data model.
type MondooProviderModel struct {
	Credentials types.String `tfsdk:"credentials"`
	Space       types.String `tfsdk:"space"`
	Region      types.String `tfsdk:"region"`
	Endpoint    types.String `tfsdk:"endpoint"`
}

func (p *MondooProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "mondoo"
	resp.Version = p.version
}

func (p *MondooProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"credentials": schema.StringAttribute{
				MarkdownDescription: "The contents of a service account key file in JSON format.",
				Optional:            true,
			},
			"space": schema.StringAttribute{
				MarkdownDescription: "The default space to manage resources in.",
				Optional:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The default region to manage resources in. Valid regions are `us` or `eu`.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("us", "eu"),
				},
			},
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "The endpoint url of the server to manage resources.",
				Optional:            true,
			},
		},
	}
}

func (p *MondooProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var data MondooProviderModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Client configuration for data sources and resources
	opts := []option.ClientOption{}

	// set the credentials to communicate with Mondoo Platform
	// 1. via MONDOO_CONFIG_BASE64
	// 2. via MONDOO_CONFIG_PATH
	// 3. via MONDOO_API_TOKEN
	// 4. via `credentials` field
	// 5. via default Mondoo CLI configuration file
	configBase64 := os.Getenv("MONDOO_CONFIG_BASE64")
	configPath := os.Getenv("MONDOO_CONFIG_PATH")
	token := os.Getenv("MONDOO_API_TOKEN")

	if configBase64 != "" {
		// extract Base64 encoded string
		data, err := base64.StdEncoding.DecodeString(configBase64)
		if err != nil {
			resp.Diagnostics.AddError(
				"MONDOO_CONFIG_BASE64 must be a valid service account",
				err.Error(),
			)
			return
		}
		opts = append(opts, option.WithServiceAccount(data))
		ctx = tflog.SetField(ctx, "env_config_base64", true)
	} else if configPath != "" {
		ctx = tflog.SetField(ctx, "env_config_path", true)
		if conf, err := parseWIF(configPath); err == nil {
			ctx = tflog.SetField(ctx, "wif", true)
			serviceAccount, err := serviceAccountFromWIFConfig(conf)
			if err != nil {
				resp.Diagnostics.AddError("Unable to exchange external token (WIF)", err.Error())
				return
			}
			opts = append(opts, option.WithServiceAccount(serviceAccount))
		} else {
			opts = append(opts, option.WithServiceAccountFile(configPath))
		}
	} else if token != "" {
		opts = append(opts, option.WithAPIToken(token))
		ctx = tflog.SetField(ctx, "env_api_token", true)
	} else if data.Credentials.ValueString() != "" {
		opts = append(opts, option.WithServiceAccount([]byte(data.Credentials.ValueString())))
		ctx = tflog.SetField(ctx, "field_credentials", true)
	} else {
		ctx = tflog.SetField(ctx, "default_cli_config_file", true)
		// if no option was provided, try the default location of Mondoo CLI configuration file
		defaultConfigPath, err := detectDefaultConfig()
		if err != nil {
			tflog.Debug(ctx, err.Error())
			resp.Diagnostics.AddError("No authentication found",
				"MONDOO_API_TOKEN, MONDOO_CONFIG_PATH or MONDOO_CONFIG_BASE64 need to be set.\n\n"+
					"To create a service account, see https://mondoo.com/docs/platform/maintain/access/service_accounts/",
			)
			return
		}
		if conf, err := parseWIF(defaultConfigPath); err == nil {
			ctx = tflog.SetField(ctx, "wif", true)
			serviceAccount, err := serviceAccountFromWIFConfig(conf)
			if err != nil {
				resp.Diagnostics.AddError("Unable to exchange external token (WIF)", err.Error())
				return
			}
			opts = append(opts, option.WithServiceAccount(serviceAccount))
		} else {
			opts = append(opts, option.WithServiceAccountFile(defaultConfigPath))
		}
	}
	tflog.Debug(ctx, "Detected authentication credentials")

	// allow the override of the endpoint
	// 1. via MONDOO_API_ENDPOINT
	// 2. via endpoint config
	// 3. via region config
	apiEndpoint := os.Getenv("MONDOO_API_ENDPOINT")
	if apiEndpoint != "" {
		url := apiEndpoint
		if !strings.HasSuffix(url, "/query") {
			url = url + "/query"
		}
		opts = append(opts, option.WithEndpoint(url))
		ctx = tflog.SetField(ctx, "env_api_endpoint", true)
	} else if data.Endpoint.ValueString() != "" {
		url := data.Endpoint.ValueString()
		if !strings.HasSuffix(url, "/query") {
			url = url + "/query"
		}
		opts = append(opts, option.WithEndpoint(url))
		ctx = tflog.SetField(ctx, "field_endpoint", true)
	} else if data.Region.ValueString() != "" {
		switch data.Region.ValueString() {
		case "eu":
			opts = append(opts, option.UseEURegion())
		case "us":
			opts = append(opts, option.UseUSRegion())
		}
		ctx = tflog.SetField(ctx, "field_region", true)
	}

	space := data.Space.ValueString()
	if space != "" {
		ctx = tflog.SetField(ctx, "provider_space", space)
	}

	tflog.Debug(ctx, "Creating Mondoo client")
	client, err := mondoov1.NewClient(opts...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to create Mondoo client",
			err.Error(),
		)
		return
	}

	// The extended GraphQL client allows us to pass additional information to
	// resources and data sources, things like the Mondoo space
	extendedClient := &ExtendedGqlClient{client, SpaceFrom(space)}
	resp.DataSourceData = extendedClient
	resp.ResourceData = extendedClient
}

func (p *MondooProvider) Resources(_ context.Context) []func() resource.Resource {
	return append(autoGeneratedResources, []func() resource.Resource{
		NewSpaceResource,
		NewServiceAccountResource,
		NewRegistrationTokenResource,
		NewCustomPolicyResource,
		NewPolicyAssigmentResource,
		NewCustomQueryPackResource,
		NewQueryPackAssigmentResource,
		NewScimGroupMappingResource,
		NewFrameworkAssignmentResource,
		NewCustomFrameworkResource,
		NewExceptionResource,
		NewIAMWorkloadIdentityBindingResource,
		NewWorkspaceResource,
		NewOrganizationResource,
		NewTeamResource,
		NewTeamExternalGroupMappingResource,
		NewIAMBindingResource,
		NewExportGSCBucketResource,
	}...)
}

func (p *MondooProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewOrganizationDataSource,
		NewSpaceDataSource,
		NewPoliciesDataSource,
		NewAssetsDataSource,
		NewFrameworksDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &MondooProvider{
			version: version,
		}
	}
}

// detectDefaultConfig tries to detect the default Mondoo CLI configuration file.
func detectDefaultConfig() (string, error) {
	f := cnquery_config.DefaultConfigFile
	homeConfig, err := cnquery_config.HomePath(f)
	if err != nil {
		return "", errors.New("failed to detect mondoo config")
	}
	if cnquery_config.ProbeFile(homeConfig) {
		return homeConfig, nil
	}

	sysConfig := cnquery_config.SystemConfigPath(f)
	if cnquery_config.ProbeFile(sysConfig) {
		return sysConfig, nil
	}

	return "", errors.New("no mondoo config found")
}

type wif struct {
	UniverseDomain   string   `json:"universeDomain"`
	Scopes           []string `json:"scopes"`
	Type             string   `json:"type"`
	Audience         string   `json:"audience"`
	SubjectTokenType string   `json:"subjectTokenType"`
	IssuerURI        string   `json:"issuerUri"`
	JWTToken         string   `json:"jwtToken"`
}

func parseWIF(filename string) (*wif, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var w wif
	err = json.Unmarshal(data, &w)
	if err != nil {
		return nil, err
	}

	return &w, nil
}

func serviceAccountFromWIFConfig(config *wif) ([]byte, error) {
	svcAccount, err := cnquery_upstream.ExchangeExternalToken(config.UniverseDomain, config.Audience, config.IssuerURI, config.JWTToken)
	if err != nil {
		return nil, err
	}
	return json.Marshal(svcAccount)
}
