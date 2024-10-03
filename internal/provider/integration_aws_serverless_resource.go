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
	Mrn   types.String `tfsdk:"mrn"`
	Name  types.String `tfsdk:"name"`
	Token types.String `tfsdk:"token"`

	Region            types.String           `tfsdk:"region"`
	ScanConfiguration ScanConfigurationInput `tfsdk:"scan_configuration"`

	// (Optional.)
	AccountIDs types.List `tfsdk:"account_ids"`
	// (Optional.)
	IsOrganization types.Bool `tfsdk:"is_organization"`

	// (Optional.)
	ConsoleSignInTrigger types.Bool `tfsdk:"console_sign_in_trigger"`
	// (Optional.)
	InstanceStateChangeTrigger types.Bool `tfsdk:"instance_state_change_trigger"`
}

type ScanConfigurationInput struct {
	// (Optional.)
	Ec2Scan types.Bool `tfsdk:"ec2_scan"`
	// (Optional.)
	EcrScan types.Bool `tfsdk:"ecr_scan"`
	// (Optional.)
	EcsScan types.Bool `tfsdk:"ecs_scan"`
	// (Optional.)
	CronScaninHours types.Int64 `tfsdk:"cron_scan_in_hours"`
	// (Optional.)
	EventScanTriggers *[]*AWSEventPatternInput `tfsdk:"event_scan_triggers"`
	// (Optional.)
	Ec2ScanOptions *Ec2ScanOptionsInput `tfsdk:"ec2_scan_options"`
	// (Optional.)
	VpcConfiguration *VPCConfigurationInput `tfsdk:"vpc_configuration"`
}

type VPCConfigurationInput struct {
	// (Optional.)
	UseDefaultVPC types.Bool `tfsdk:"use_default_vpc"`
	// (Optional.)
	UseMondooVPC types.Bool `tfsdk:"use_mondoo_vpc"`
	// (Optional.)
	CIDR types.String `tfsdk:"cidr_block"`
}

type AWSEventPatternInput struct {
	// (Required.)
	ScanType types.String `tfsdk:"scan_type"`
	// (Required.)
	EventSource types.String `tfsdk:"event_source"`
	// (Required.)
	EventDetailType types.String `tfsdk:"event_detail_type"`
}

type Ec2ScanOptionsInput struct {
	// (Optional.)
	Ssm types.Bool `tfsdk:"ssm"`
	// (Optional.)
	InstanceIDsFilter types.List `tfsdk:"instance_ids_filter"`
	// (Optional.)
	RegionsFilter types.List `tfsdk:"regions_filter"`
	// (Optional.)
	TagsFilter types.Map `tfsdk:"tags_filter"`
	// (Optional.)
	EbsVolumeScan types.Bool `tfsdk:"ebs_volume_scan"`
	// (Optional.)
	EbsScanOptions *EbsScanOptionsInput `tfsdk:"ebs_scan_options"`
	// (Optional.)
	InstanceConnect types.Bool `tfsdk:"instance_connect"`
}

type EbsScanOptionsInput struct {
	// (Optional.)
	TargetInstancesPerScanner types.Int64 `tfsdk:"target_instances_per_scanner"`
	// (Optional.)
	MaxAsgInstances types.Int64 `tfsdk:"max_asg_instances"`
}

func (m integrationAwsServerlessResourceModel) GetConfigurationOptions() *mondoov1.AWSConfigurationOptionsInput {
	var opts *mondoov1.AWSConfigurationOptionsInput
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

	var accountIDs []mondoov1.String
	accountIds, _ := m.AccountIDs.ToListValue(context.Background())
	accountIds.ElementsAs(context.Background(), &accountIDs, true)

	opts = &mondoov1.AWSConfigurationOptionsInput{
		Region:         mondoov1.String(m.Region.ValueString()),
		IsOrganization: mondoov1.NewBooleanPtr(mondoov1.Boolean(m.IsOrganization.ValueBool())),
		AccountIDs:     &accountIDs,
		ScanConfiguration: mondoov1.ScanConfigurationInput{
			VpcConfiguration: &mondoov1.VPCConfigurationInput{
				UseDefaultVPC: mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.VpcConfiguration.UseDefaultVPC.ValueBool())),
				UseMondooVPC:  mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.VpcConfiguration.UseMondooVPC.ValueBool())),
				CIDR:          mondoov1.NewStringPtr(mondoov1.String(m.ScanConfiguration.VpcConfiguration.CIDR.ValueString())),
			},
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
			"token": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Integration token",
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
					"cron_scan_in_hours": schema.Int64Attribute{
						MarkdownDescription: "Cron scan in hours.",
						Optional:            true,
					},
					"vpc_configuration": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"use_default_vpc": schema.BoolAttribute{
								MarkdownDescription: "Use default VPC.",
								Optional:            true,
							},
							"use_mondoo_vpc": schema.BoolAttribute{
								MarkdownDescription: "Use Mondoo VPC.",
								Optional:            true,
							},
							"cidr_block": schema.StringAttribute{
								MarkdownDescription: "CIDR block for the Mondoo VPC.",
								Optional:            true,
							},
						},
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
					"event_scan_triggers": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"scan_type": schema.StringAttribute{
								MarkdownDescription: "Scan type.",
								Optional:            true,
							},
							"event_source": schema.StringAttribute{
								MarkdownDescription: "Event source.",
								Optional:            true,
							},
							"event_detail_type": schema.StringAttribute{
								MarkdownDescription: "Event detail type.",
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

func (r integrationAwsServerlessResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data integrationAwsServerlessResourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// user has provided both default or mondoo vpc
	if !data.ScanConfiguration.VpcConfiguration.UseDefaultVPC.IsNull() && !data.ScanConfiguration.VpcConfiguration.UseMondooVPC.IsNull() {
		defaultVpc := data.ScanConfiguration.VpcConfiguration.UseDefaultVPC.ValueBool()
		mondooVpc := data.ScanConfiguration.VpcConfiguration.UseMondooVPC.ValueBool()
		if defaultVpc && mondooVpc {
			resp.Diagnostics.AddError(
				"ConflictingAttributesError",
				"Cannot set both use_default_vpc and use_mondoo_vpc to true at the same time.",
			)
		}

		if !defaultVpc && !mondooVpc {
			resp.Diagnostics.AddError(
				"ConflictingAttributesError",
				"Cannot set both use_default_vpc and use_mondoo_vpc to false at the same time.",
			)
		}
	}
	// user has provided mondoo vpc only
	if mondooVpc := data.ScanConfiguration.VpcConfiguration.UseMondooVPC.ValueBool(); mondooVpc {
		if cidr := data.ScanConfiguration.VpcConfiguration.CIDR.ValueString(); cidr == "" {
			resp.Diagnostics.AddError(
				"MissingAttributeError",
				"Attribute cidr_block must not be empty when use_mondoo_vpc is set to true.",
			)
		}
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

	var accountIDs []mondoov1.String
	accountIds, _ := data.AccountIDs.ToListValue(context.Background())
	accountIds.ElementsAs(context.Background(), &accountIDs, true)

	// Check if both whitelist and blacklist are provided
	if len(accountIDs) > 0 && data.IsOrganization.ValueBool() {
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
	data.Token = types.StringValue(string(integration.Token))

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

	var accountIDs []mondoov1.String
	accountIds, _ := data.AccountIDs.ToListValue(context.Background())
	accountIds.ElementsAs(context.Background(), &accountIDs, true)

	// Check if both whitelist and blacklist are provided
	if len(accountIDs) > 0 && data.IsOrganization.ValueBool() {
		resp.Diagnostics.AddError("ConflictingAttributesError", "Cannot install CloudFormation Stack to both AWS organization and accounts.")
		return
	}

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

	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete AWS serverless integration '%s', got error: %s", data.Mrn.ValueString(), err.Error()))
		return
	}
}

func (r *integrationAwsServerlessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("mrn"), req, resp)
}
