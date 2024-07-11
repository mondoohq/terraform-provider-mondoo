// Copyright (c) Mondoo, Inc.
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/base64"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
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

func (p *MondooProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "mondoo"
	resp.Version = p.version
}

func (p *MondooProvider) Schema(ctx context.Context, req provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"credentials": schema.StringAttribute{
				MarkdownDescription: "Either the path to or the contents of a service account key file in JSON format.",
				Optional:            true,
			},
			"space": schema.StringAttribute{
				MarkdownDescription: "The default space to manage resources in.",
				Optional:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The default region to manage resources in.",
				Optional:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("us", "eu"),
				},
			},
			"endpoint": schema.StringAttribute{
				MarkdownDescription: "The endpoint url of the server to manage resources",
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
	configBase64 := os.Getenv("MONDOO_CONFIG_BASE64")
	configPath := os.Getenv("MONDOO_CONFIG_PATH")
	token := os.Getenv("MONDOO_API_TOKEN")

	if configBase64 != "" {
		// extract base 64 encoded string
		data, err := base64.StdEncoding.DecodeString(configBase64)
		if err != nil {
			resp.Diagnostics.AddError(
				"MONDOO_CONFIG_BASE64 must be a valid service account",
				err.Error(),
			)
			return
		}
		opts = append(opts, option.WithServiceAccount(data))
	} else if configPath != "" {
		opts = append(opts, option.WithServiceAccountFile(configPath))
	} else if token != "" {
		opts = append(opts, option.WithAPIToken(token))
	}

	if len(opts) == 0 {
		resp.Diagnostics.AddError(
			"MONDOO_API_TOKEN, MONDOO_CONFIG_PATH or MONDOO_CONFIG_BASE64 need to be set",
			"MONDOO_API_TOKEN, MONDOO_CONFIG_PATH or MONDOO_CONFIG_BASE64 need to be set",
		)
		return
	}

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
	} else if data.Endpoint.ValueString() != "" {
		url := data.Endpoint.ValueString()
		if !strings.HasSuffix(url, "/query") {
			url = url + "/query"
		}
		opts = append(opts, option.WithEndpoint(url))
	} else if data.Region.ValueString() != "" {
		switch data.Region.ValueString() {
		case "eu":
			opts = append(opts, option.UseEURegion())
		case "us":
			opts = append(opts, option.UseUSRegion())
		}
	}

	client, err := mondoov1.NewClient(opts...)
	if err != nil {
		resp.Diagnostics.AddError(
			"Failed to create Mondoo client",
			err.Error(),
		)
		return
	}
	resp.DataSourceData = client
	resp.ResourceData = client
}

func (p *MondooProvider) Resources(ctx context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewSpaceResource,
		NewServiceAccountResource,
		NewRegistrationTokenResource,
		NewCustomPolicyResource,
		NewPolicyAssigmentResource,
		NewCustomQueryPackResource,
		NewQueryPackAssigmentResource,
		NewScimGroupMappingResource,
		NewIntegrationAzureResource,
		NewIntegrationAwsResource,
		NewIntegrationDomainResource,
		NewIntegrationGcpResource,
		NewIntegrationOciTenantResource,
		NewIntegrationSlackResource,
		NewIntegrationMs365Resource,
		NewIntegrationGithubResource,
		NewComplianceFrameworkResource,
		NewCustomComplianceFrameworkResource,
	}
}

func (p *MondooProvider) DataSources(ctx context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewOrganizationDataSource,
		NewSpaceDataSource,
		NewPolicyDataSource,
		NewAssetsDataSource,
		NewComplianceFrameworkDataSource,
	}
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &MondooProvider{
			version: version,
		}
	}
}
