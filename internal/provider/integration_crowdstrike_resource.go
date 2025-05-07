package provider

import (
	"context"
	"fmt"

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

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = (*integrationCrowdstrikeResource)(nil)
var _ resource.ResourceWithImportState = (*integrationCrowdstrikeResource)(nil)

func NewIntegrationCrowdstrikeResource() resource.Resource {
	return &integrationCrowdstrikeResource{}
}

type integrationCrowdstrikeResource struct {
	client *ExtendedGqlClient
}

type integrationCrowdstrikeResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn          types.String `tfsdk:"mrn"`
	Name         types.String `tfsdk:"name"`
	ClientId     types.String `tfsdk:"client_id"`
	ClientSecret types.String `tfsdk:"client_secret"`
	Cloud        types.String `tfsdk:"cloud"`
	MemberCID    types.String `tfsdk:"member_cid"`
}

func (m integrationCrowdstrikeResourceModel) GetConfigurationOptions() *mondoov1.CrowdstrikeFalconConfigurationOptionsInput {
	return &mondoov1.CrowdstrikeFalconConfigurationOptionsInput{
		ClientID:     mondoov1.String(m.ClientId.ValueString()),
		ClientSecret: mondoov1.String(m.ClientSecret.ValueString()),
		Cloud:        mondoov1.NewStringPtr(mondoov1.String(m.Cloud.ValueString())),
		MemberCID:    mondoov1.NewStringPtr(mondoov1.String(m.MemberCID.ValueString())),
	}
}

func (r *integrationCrowdstrikeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_crowdstrike"
}

func (r *integrationCrowdstrikeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "CrowdStrike Falcon for Cloud integration.",
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
			"client_id": schema.StringAttribute{
				MarkdownDescription: "Client ID used for authentication with CrowdStrike Falcon platform.",
				Required:            true,
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "Client Secret used for authentication with CrowdStrike Falcon platform.",
				Required:            true,
				Sensitive:           true,
			},
			"cloud": schema.StringAttribute{
				MarkdownDescription: "The Falcon Cloud to connect.",
				Optional:            true,
			},
			"member_cid": schema.StringAttribute{
				MarkdownDescription: "CID selector for cases when the client ID and secret has access to multiple CIDs.",
				Optional:            true,
			},
		},
	}
}

func (r *integrationCrowdstrikeResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationCrowdstrikeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationCrowdstrikeResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

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
		mondoov1.ClientIntegrationTypeCrowdstrikeFalcon,
		mondoov1.ClientIntegrationConfigurationInput{
			CrowdstrikeFalconConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create %s integration. Got error: %s", mondoov1.IntegrationTypeCrowdstrikeFalcon, err),
			)
		return
	}

	// trigger integration to gather results quickly after the first setup
	// NOTE: we ignore the error since the integration state does not depend on it
	_, err = r.client.TriggerAction(ctx, string(integration.Mrn), mondoov1.ActionTypeRunImport)
	if err != nil {
		resp.Diagnostics.
			AddWarning("Client Error",
				fmt.Sprintf("Unable to trigger integration. Got error: %s", err),
			)
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))
	data.SpaceID = types.StringValue(space.ID())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationCrowdstrikeResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationCrowdstrikeResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationCrowdstrikeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationCrowdstrikeResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		CrowdstrikeFalconConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeCrowdstrikeFalcon,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update %s integration. Got error: %s", mondoov1.IntegrationTypeCrowdstrikeFalcon, err),
			)
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationCrowdstrikeResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationCrowdstrikeResourceModel

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
				fmt.Sprintf("Unable to delete %s integration. Got error: %s", mondoov1.IntegrationTypeCrowdstrikeFalcon, err),
			)
		return
	}
}

func (r *integrationCrowdstrikeResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	model := integrationCrowdstrikeResourceModel{
		Mrn:          types.StringValue(integration.Mrn),
		Name:         types.StringValue(integration.Name),
		SpaceID:      types.StringValue(integration.SpaceID()),
		ClientId:     types.StringValue(integration.ConfigurationOptions.CrowdstrikeFalconConfigurationOptionsInput.ClientId),
		ClientSecret: types.StringPointerValue(nil),
		Cloud:        types.StringValue(integration.ConfigurationOptions.CrowdstrikeFalconConfigurationOptionsInput.Cloud),
		MemberCID:    types.StringValue(integration.ConfigurationOptions.CrowdstrikeFalconConfigurationOptionsInput.MemberCID),
	}

	resp.State.Set(ctx, &model)
}
