package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var _ resource.Resource = (*integrationAwsServerlessResource)(nil)

func NewIntegrationAwsServerlessResource() resource.Resource {
	return &integrationAwsServerlessResource{}
}

type integrationAwsServerlessResource struct {
	client *ExtendedGqlClient
}

type integrationAwsServerlessResourceModel struct {
	// scope
	SpaceId types.String `tfsdk:"space_id"`

	// integration details
	Mrn  types.String `tfsdk:"mrn"`
	Name types.String `tfsdk:"name"`

	Region            types.String           `json:"region"`
	ScanConfiguration ScanConfigurationInput `json:"scanConfiguration"`

	// (Optional.)
	AccountIDs []types.String `json:"accountIds,omitempty"`
	// (Optional.)
	IsOrganization types.Bool `json:"isOrganization,omitempty"`

	// (Optional.)
	ConsoleSignInTrigger types.Bool `json:"consoleSignInTrigger,omitempty"`
	// (Optional.)
	InstanceStateChangeTrigger types.Bool `json:"instanceStateChangeTrigger,omitempty"`
}

type ScanConfigurationInput struct {
	// (Optional.)
	AccountScan types.Bool `json:"accountScan,omitempty"`
	// (Optional.)
	Ec2Scan types.Bool `json:"ec2Scan,omitempty"`
	// (Optional.)
	EcrScan types.Bool `json:"ecrScan,omitempty"`
	// (Optional.)
	EcsScan types.Bool `json:"ecsScan,omitempty"`
	// (Optional.)
	CronScaninHours types.Int64 `json:"cronScaninHours,omitempty"`
	// (Optional.)
	EventScanTriggers *[]*AWSEventPatternInput `json:"eventScanTriggers,omitempty"`
	// (Optional.)
	Ec2ScanOptions *Ec2ScanOptionsInput `json:"ec2ScanOptions,omitempty"`
}

type AWSEventPatternInput struct {
	// (Required.)
	ScanType types.String `json:"scanType"`
	// (Required.)
	EventSource types.String `json:"eventSource"`
	// (Required.)
	EventDetailType types.String `json:"eventDetailType"`
}

// Ec2ScanOptionsInput
type Ec2ScanOptionsInput struct {
	// (Optional.)
	Ssm types.Bool `json:"ssm,omitempty"`
	// (Optional.)
	InstanceIDsFilter types.List `json:"instanceIdsFilter,omitempty"`
	// (Optional.)
	RegionsFilter types.List `json:"regionsFilter,omitempty"`
	// (Optional.)
	TagsFilter types.Map `json:"tagsFilter,omitempty"`
	// (Optional.)
	EbsVolumeScan types.Bool `json:"ebsVolumeScan,omitempty"`
	// (Optional.)
	EbsScanOptions *EbsScanOptionsInput `json:"ebsScanOptions,omitempty"`
	// (Optional.)
	InstanceConnect types.Bool `json:"instanceConnect,omitempty"`
}

// EbsScanOptionsInput
type EbsScanOptionsInput struct {
	// (Optional.)
	TargetInstancesPerScanner types.Int64 `json:"targetInstancesPerScanner,omitempty"`
	// (Optional.)
	MaxAsgInstances types.Int64 `json:"maxAsgInstances,omitempty"`
}

func (m integrationAwsServerlessResourceModel) GetConfigurationOptions() *mondoov1.AWSConfigurationOptionsInput {
	opts := &mondoov1.AWSConfigurationOptionsInput{}
	var eventScanTriggers []*mondoov1.AWSEventPatternInput

	if m.InstanceStateChangeTrigger.ValueBool() && m.ConsoleSignInTrigger.ValueBool() {
		eventScanTriggers = append(eventScanTriggers, &mondoov1.AWSEventPatternInput{
			ScanType:        mondoov1.String("ALL"),
			EventSource:     mondoov1.String("aws.signin"),
			EventDetailType: mondoov1.String("AWS Console Sign In via CloudTrail"),
		})
		eventScanTriggers = append(eventScanTriggers, &mondoov1.AWSEventPatternInput{
			ScanType:        mondoov1.String("ALL"),
			EventSource:     mondoov1.String("aws.ec2"),
			EventDetailType: mondoov1.String("EC2 Instance State-change Notification"),
		})
	} else if m.ConsoleSignInTrigger.ValueBool() && !m.InstanceStateChangeTrigger.ValueBool() {
		eventScanTriggers = append(eventScanTriggers, &mondoov1.AWSEventPatternInput{
			ScanType:        mondoov1.String("ALL"),
			EventSource:     mondoov1.String("aws.signin"),
			EventDetailType: mondoov1.String("AWS Console Sign In via CloudTrail"),
		})
	} else if m.InstanceStateChangeTrigger.ValueBool() && !m.ConsoleSignInTrigger.ValueBool() {
		eventScanTriggers = append(eventScanTriggers, &mondoov1.AWSEventPatternInput{
			ScanType:        mondoov1.String("ALL"),
			EventSource:     mondoov1.String("aws.ec2"),
			EventDetailType: mondoov1.String("EC2 Instance State-change Notification"),
		})
	}

	var instanceIdsFilter []mondoov1.String
	instanceIds, _ := m.ScanConfiguration.Ec2ScanOptions.InstanceIDsFilter.ToListValue(context.Background())
	instanceIds.ElementsAs(context.Background(), &instanceIdsFilter, true)

	var RegionsFilter []mondoov1.String
	regions, _ := m.ScanConfiguration.Ec2ScanOptions.RegionsFilter.ToListValue(context.Background())
	regions.ElementsAs(context.Background(), &RegionsFilter, true)

	var tagsFilter mondoov1.Map
	tags, _ := m.ScanConfiguration.Ec2ScanOptions.TagsFilter.ToMapValue(context.Background())
	tags.ElementsAs(context.Background(), &tagsFilter, true)

	opts = &mondoov1.AWSConfigurationOptionsInput{
		Region: mondoov1.String(m.Region.ValueString()),
		ScanConfiguration: mondoov1.ScanConfigurationInput{
			AccountScan:       mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.AccountScan.ValueBool())),
			Ec2Scan:           mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.Ec2Scan.ValueBool())),
			EcrScan:           mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.EcrScan.ValueBool())),
			EcsScan:           mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.EcsScan.ValueBool())),
			CronScaninHours:   mondoov1.NewIntPtr(mondoov1.Int(m.ScanConfiguration.CronScaninHours.ValueInt64())),
			EventScanTriggers: &eventScanTriggers,
			Ec2ScanOptions: &mondoov1.Ec2ScanOptionsInput{
				Ssm:               mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.Ec2ScanOptions.Ssm.ValueBool())),
				InstanceIDsFilter: &instanceIdsFilter,
				RegionsFilter:     &RegionsFilter,
				TagsFilter:        &tagsFilter,
				EbsVolumeScan:     mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.Ec2ScanOptions.EbsVolumeScan.ValueBool())),
				EbsScanOptions: &mondoov1.EbsScanOptionsInput{
					TargetInstancesPerScanner: mondoov1.NewIntPtr(mondoov1.Int(m.ScanConfiguration.Ec2ScanOptions.EbsScanOptions.TargetInstancesPerScanner.ValueInt64())),
					MaxAsgInstances:           mondoov1.NewIntPtr(mondoov1.Int(m.ScanConfiguration.Ec2ScanOptions.EbsScanOptions.MaxAsgInstances.ValueInt64())),
				},
				InstanceConnect: mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.Ec2ScanOptions.InstanceConnect.ValueBool())),
			},
		},
	}

	return opts
}

func (r *integrationAwsServerlessResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_aws_serverless"
}

func (r *integrationAwsServerlessResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan AWS organization and accounts for misconfigurations and vulnerabilities.`,
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
			},
			"region": schema.StringAttribute{
				MarkdownDescription: "AWS region.",
				Required:            true,
			},
			"console_sign_in_trigger": schema.BoolAttribute{
				MarkdownDescription: "Enable console sign in trigger.",
				Optional:            true,
			},
			"instance_state_change_trigger": schema.BoolAttribute{
				MarkdownDescription: "Enable instance state change trigger.",
				Optional:            true,
			},
			"scan_configuration": schema.SingleNestedAttribute{
				Required: true,
				Attributes: map[string]schema.Attribute{
					"account_scan": schema.BoolAttribute{
						MarkdownDescription: "Enable account scan.",
						Optional:            true,
					},
					"ec2_scan": schema.BoolAttribute{
						MarkdownDescription: "Enable EC2 scan.",
						Optional:            true,
					},
					"ecr_scan": schema.BoolAttribute{
						MarkdownDescription: "Enable ECR scan.",
						Optional:            true,
					},
					"ecs_scan": schema.BoolAttribute{
						MarkdownDescription: "Enable ECS scan.",
						Optional:            true,
					},
					"cron_scanin_hours": schema.Int64Attribute{
						MarkdownDescription: "Cron scan in hours.",
						Optional:            true,
					},
					"ec2_scan_options": schema.SingleNestedAttribute{
						Required: true,
						Attributes: map[string]schema.Attribute{
							"ssm": schema.BoolAttribute{
								MarkdownDescription: "Enable SSM.",
								Optional:            true,
							},
							"instance_ids_filter": schema.ListAttribute{
								MarkdownDescription: "List of instance IDs filter.",
								Optional:            true,
								ElementType:         types.StringType,
							},
							"regions_filter": schema.ListAttribute{
								MarkdownDescription: "List of regions filter.",
								Optional:            true,
								ElementType:         types.StringType,
							},
							"tags_filter": schema.MapAttribute{
								MarkdownDescription: "Tags filter.",
								Optional:            true,
								ElementType:         types.StringType,
							},
							"ebs_volume_scan": schema.BoolAttribute{
								MarkdownDescription: "Enable EBS volume scan.",
								Optional:            true,
							},
							"ebs_scan_options": schema.SingleNestedAttribute{
								Required: true,
								Attributes: map[string]schema.Attribute{
									"target_instances_per_scanner": schema.Int64Attribute{
										MarkdownDescription: "Target instances per scanner.",
										Optional:            true,
									},
									"max_asg_instances": schema.Int64Attribute{
										MarkdownDescription: "Max ASG instances.",
										Optional:            true,
									},
								},
							},
							"instance_connect": schema.BoolAttribute{
								MarkdownDescription: "Enable instance connect.",
								Optional:            true,
							},
						},
					},
				},
			},
			"account_ids": schema.ListAttribute{
				MarkdownDescription: "List of AWS account IDs.",
				Optional:            true,
				ElementType:         types.StringType,
			},
			"is_organization": schema.BoolAttribute{
				MarkdownDescription: "Is organization.",
				Optional:            true,
			},
		},
	}
}

func (r *integrationAwsServerlessResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationAwsServerlessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationAwsServerlessResourceModel

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

	// Check if both whitelist and blacklist are provided
	if len(data.AccountIDs) > 0 && data.IsOrganization.ValueBool() {
		resp.Diagnostics.AddError("ConflictingAttributesError", "Cannot install CloudFormation Stack to both AWS organization and accounts.")
		return
	}

	integration, err := r.client.CreateIntegration(ctx,
		spaceMrn,
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAws,
		mondoov1.ClientIntegrationConfigurationInput{
			AwsConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create AWS integration, got error: %s", err))
		return
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))
	data.SpaceId = types.StringValue(data.SpaceId.ValueString())

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAwsServerlessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationAwsServerlessResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Read API call logic

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAwsServerlessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationAwsServerlessResourceModel

	// Read Terraform plan data into the model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		AwsConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx, data.Mrn.ValueString(), data.Name.ValueString(), mondoov1.ClientIntegrationTypeAws, opts)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update AWS integration, got error: %s", err))
		return
	}

	// Save updated data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationAwsServerlessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationAwsServerlessResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Delete API call logic
}

func (r *integrationAwsServerlessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("mrn"), req, resp)
}
