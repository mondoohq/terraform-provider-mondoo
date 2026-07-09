// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
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
var (
	_ resource.Resource                = (*integrationGcpServerlessResource)(nil)
	_ resource.ResourceWithImportState = (*integrationGcpServerlessResource)(nil)
)

func NewIntegrationGcpServerlessResource() resource.Resource {
	return &integrationGcpServerlessResource{}
}

type integrationGcpServerlessResource struct {
	client *ExtendedGqlClient
}

type integrationGcpServerlessResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn   types.String `tfsdk:"mrn"`
	Name  types.String `tfsdk:"name"`
	Token types.String `tfsdk:"token"`

	// The GCP scope to scan (a folder ID or an organization ID).
	Scope types.String `tfsdk:"scope"`
	// The GCP project ID where the serverless scanner is deployed.
	HostProjectID types.String `tfsdk:"host_project_id"`
	// The GCP region where the serverless scanner is deployed.
	Region types.String `tfsdk:"region"`
	// (Optional.) A customer-provided service account identity to run the
	// integration with, instead of the platform creating one.
	SuppliedSaIdentity types.String `tfsdk:"supplied_sa_identity"`

	// (Optional.)
	ScanConfiguration *GcpServerlessScanConfigurationInput `tfsdk:"scan_configuration"`
}

type GcpServerlessScanConfigurationInput struct {
	// (Optional.) Include filter: only projects whose tags match at least one of
	// these key-value pairs are scanned. A value of "*" matches any value for the key.
	TagsFilter types.Map `tfsdk:"tags_filter"`
	// (Optional.) Exclude filter: projects whose tags match at least one of these
	// key-value pairs are skipped. A value of "*" matches any value for the key.
	ExcludedTagsFilter types.Map `tfsdk:"excluded_tags_filter"`
	// (Optional.) How often (in hours) the deployed scanner runs a scan.
	ScanScheduleHours types.Int32 `tfsdk:"scan_schedule_hours"`
}

// tagsFilterToKeyValueList converts a Terraform string map into the GraphQL
// list of key-value pairs expected by the API. It returns nil when the map is
// empty so the field is omitted from the request.
func tagsFilterToKeyValueList(m types.Map) *[]mondoov1.KeyValueInput {
	if m.IsNull() || m.IsUnknown() || len(m.Elements()) == 0 {
		return nil
	}

	kvs := make([]mondoov1.KeyValueInput, 0, len(m.Elements()))
	for k, v := range m.Elements() {
		value := strings.Trim(v.String(), "\"")
		kvs = append(kvs, mondoov1.KeyValueInput{
			Key:   mondoov1.String(k),
			Value: mondoov1.NewStringPtr(mondoov1.String(value)),
		})
	}
	return &kvs
}

func (m integrationGcpServerlessResourceModel) GetConfigurationOptions() *mondoov1.GcpServerlessConfigurationOptionsInput {
	opts := &mondoov1.GcpServerlessConfigurationOptionsInput{
		HostProjectId: mondoov1.String(m.HostProjectID.ValueString()),
		Region:        mondoov1.String(m.Region.ValueString()),
	}

	// Omitted scope means the scanner falls back to its default scope.
	if scope := m.Scope.ValueString(); scope != "" {
		opts.Scope = mondoov1.NewStringPtr(mondoov1.String(scope))
	}

	// Omitted means the platform creates the integration's identity itself.
	if sa := m.SuppliedSaIdentity.ValueString(); sa != "" {
		opts.SuppliedSaIdentity = mondoov1.NewStringPtr(mondoov1.String(sa))
	}

	if m.ScanConfiguration != nil {
		opts.ScanConfiguration = &mondoov1.GcpServerlessScanConfigurationInput{
			TagsFilter:         tagsFilterToKeyValueList(m.ScanConfiguration.TagsFilter),
			ExcludedTagsFilter: tagsFilterToKeyValueList(m.ScanConfiguration.ExcludedTagsFilter),
		}
		if !m.ScanConfiguration.ScanScheduleHours.IsNull() && !m.ScanConfiguration.ScanScheduleHours.IsUnknown() {
			opts.ScanConfiguration.ScanScheduleHours = mondoov1.NewIntPtr(mondoov1.Int(m.ScanConfiguration.ScanScheduleHours.ValueInt32()))
		}
	}

	return opts
}

func (r *integrationGcpServerlessResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_gcp_serverless"
}

func (r *integrationGcpServerlessResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan GCP organizations and folders for misconfigurations and vulnerabilities using a serverless scanner deployed into your own host project.`,
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
			"token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Integration token. Pass this to the serverless scanner deployment to register it with this integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				MarkdownDescription: "Name of the integration.",
				Required:            true,
			},
			"scope": schema.StringAttribute{
				MarkdownDescription: "The GCP scope to scan. Accepts either a folder ID or an organization ID. When omitted, the scanner falls back to its default scope.",
				Optional:            true,
			},
			"host_project_id": schema.StringAttribute{
				MarkdownDescription: "The GCP project ID where the serverless scanner is deployed.",
				Required:            true,
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "The GCP region where the serverless scanner is deployed.",
				Required:            true,
			},
			"supplied_sa_identity": schema.StringAttribute{
				MarkdownDescription: "A customer-provided service account identity to run this integration with, instead of the platform automatically creating one (bring-your-own-identity). Stored and returned verbatim.",
				Optional:            true,
			},
			"scan_configuration": schema.SingleNestedAttribute{
				MarkdownDescription: "Scan options that control what the deployed scanner scans.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"tags_filter": schema.MapAttribute{
						MarkdownDescription: "Include filter: when not empty, only projects whose tags match at least one of these key-value pairs are scanned. A value of `*` matches any value for that tag key.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"excluded_tags_filter": schema.MapAttribute{
						MarkdownDescription: "Exclude filter: projects whose tags match at least one of these key-value pairs are skipped, even if they match the include filter. A value of `*` matches any value for that tag key.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"scan_schedule_hours": schema.Int32Attribute{
						MarkdownDescription: "How often (in hours) the deployed scanner runs a scan.",
						Optional:            true,
						Validators: []validator.Int32{
							int32validator.AtLeast(1),
						},
					},
				},
			},
		},
	}
}

func (r *integrationGcpServerlessResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationGcpServerlessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationGcpServerlessResourceModel

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
		mondoov1.ClientIntegrationTypeGcpServerless,
		mondoov1.ClientIntegrationConfigurationInput{
			GcpServerlessConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create GCP serverless integration. Got error: %s", err),
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
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))
	data.Token = types.StringValue(string(integration.Token))
	data.SpaceID = types.StringValue(space.ID())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGcpServerlessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationGcpServerlessResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGcpServerlessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationGcpServerlessResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		GcpServerlessConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeGcpServerless,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update GCP serverless integration. Got error: %s", err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationGcpServerlessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationGcpServerlessResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf(
					"Unable to delete GCP serverless integration '%s'. Got error: %s",
					data.Mrn.ValueString(), err.Error(),
				),
			)
		return
	}
}

func (r *integrationGcpServerlessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	model := integrationGcpServerlessResourceModel{
		Mrn:     types.StringValue(integration.Mrn),
		Name:    types.StringValue(integration.Name),
		SpaceID: types.StringValue(integration.SpaceID()),
	}

	resp.State.Set(ctx, &model)
}
