// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/boolvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

// Ensure provider defined types fully satisfy framework interfaces.
var (
	_ resource.Resource                = (*integrationAwsServerlessResource)(nil)
	_ resource.ResourceWithImportState = (*integrationAwsServerlessResource)(nil)
)

func NewIntegrationAwsServerlessResource() resource.Resource {
	return &integrationAwsServerlessResource{}
}

type integrationAwsServerlessResource struct {
	client *ExtendedGqlClient
}

type integrationAwsServerlessResourceModel struct {
	// scope
	SpaceID types.String `tfsdk:"space_id"`

	// integration details
	Mrn   types.String `tfsdk:"mrn"`
	Name  types.String `tfsdk:"name"`
	Token types.String `tfsdk:"token"`

	// Computed by the server from its own configuration. Feed them into the
	// aws_cloudformation_stack resource: the template URL, and the
	// MondooSourceBucket parameter the template requires.
	SourceBucket              types.String `tfsdk:"source_bucket"`
	CloudFormationTemplateUrl types.String `tfsdk:"cloud_formation_template_url"`
	// The stack's MondooWifConfig parameter, for a WIF integration.
	WifConfig types.String `tfsdk:"wif_config"`

	Region            types.String           `tfsdk:"region"`
	ScanConfiguration ScanConfigurationInput `tfsdk:"scan_configuration"`

	// (Optional.)
	AccountIDs types.List `tfsdk:"account_ids"`
	// (Optional.)
	IsOrganization types.Bool `tfsdk:"is_organization"`

	// (Optional.) When true, the Lambda authenticates to Mondoo with Workload
	// Identity Federation instead of the token. Immutable after creation.
	UseWif types.Bool `tfsdk:"use_wif"`
	// (Optional.) The AWS account the Lambda is deployed in; the WIF binding's
	// subject. Required with use_wif, rejected without it. Immutable after creation.
	AwsAccountID types.String `tfsdk:"aws_account_id"`

	// (Optional.)
	ConsoleSignInTrigger types.Bool `tfsdk:"console_sign_in_trigger"`
	// (Optional.)
	InstanceStateChangeTrigger types.Bool `tfsdk:"instance_state_change_trigger"`
}

type ScanConfigurationInput struct {
	// (Optional.)
	AccountScan types.Bool `tfsdk:"account_scan"`
	// (Optional.)
	Ec2Scan types.Bool `tfsdk:"ec2_scan"`
	// (Optional.)
	EcrScan types.Bool `tfsdk:"ecr_scan"`
	// (Optional.)
	EcsScan types.Bool `tfsdk:"ecs_scan"`
	// (Optional.)
	CronScaninHours types.Int64 `tfsdk:"cron_scan_in_hours"`
	// (Optional.)
	CronScanExpression types.String `tfsdk:"cron_scan_expression"`
	// (Optional.)
	EventScanTriggers *[]*AWSEventPatternInput `tfsdk:"event_scan_triggers"`
	// (Optional.)
	Ec2ScanOptions *Ec2ScanOptionsInput `tfsdk:"ec2_scan_options"`
	// (Optional.)
	VpcConfiguration *VPCConfigurationInput `tfsdk:"vpc_configuration"`
	// (Optional.)
	CrossAccountScan types.Bool `tfsdk:"cross_account_scan"`
	// (Optional.)
	CrossAccountScanOptions *CrossAccountScanOptionsInput `tfsdk:"cross_account_scan_options"`
	// (Optional.)
	PropagateAccountTags types.Bool `tfsdk:"propagate_account_tags"`
	// (Optional.)
	DiscoveryTargets types.List `tfsdk:"discovery_targets"`
	// (Optional.)
	Scheduling types.String `tfsdk:"scheduling"`
}

type CrossAccountScanOptionsInput struct {
	// (Optional.)
	RoleName types.String `tfsdk:"role_name"`
	// (Optional.)
	AccountIDs types.List `tfsdk:"account_ids"`
	// (Optional.) Tag key to tag value.
	AccountTags types.Map `tfsdk:"account_tags"`
	// (Optional.) AWS Organization ID to its management account ID.
	Organizations types.Map `tfsdk:"organizations"`
	// (Optional.)
	EbsVolumeScan types.Bool `tfsdk:"ebs_volume_scan"`
}

// input maps the cross-account options to their API input. Unset attributes
// are omitted from the request.
func (o *CrossAccountScanOptionsInput) input() *mondoov1.AWSCrossAccountScanOptionsInput {
	in := &mondoov1.AWSCrossAccountScanOptionsInput{
		RoleName:      OptionalString(o.RoleName),
		EbsVolumeScan: OptionalBool(o.EbsVolumeScan),
	}
	if !o.AccountIDs.IsNull() && !o.AccountIDs.IsUnknown() {
		accountIDs := ConvertSliceStrings(o.AccountIDs)
		in.AccountIDs = &accountIDs
	}
	if tags, ok := stringMap(o.AccountTags); ok {
		accountTags := make([]mondoov1.TagInput, 0, len(tags))
		for _, key := range slices.Sorted(maps.Keys(tags)) {
			accountTags = append(accountTags, mondoov1.TagInput{
				Key:   mondoov1.String(key),
				Value: mondoov1.String(tags[key]),
			})
		}
		in.AccountTags = &accountTags
	}
	if orgs, ok := stringMap(o.Organizations); ok {
		organizations := make([]mondoov1.AWSCrossAccountOrganizationInput, 0, len(orgs))
		for _, orgID := range slices.Sorted(maps.Keys(orgs)) {
			organizations = append(organizations, mondoov1.AWSCrossAccountOrganizationInput{
				OrganizationId:      mondoov1.String(orgID),
				ManagementAccountId: mondoov1.String(orgs[orgID]),
			})
		}
		in.Organizations = &organizations
	}
	return in
}

// stringMap returns the elements of a map(string) attribute, and false when the
// attribute is not set in the configuration (null or unknown).
func stringMap(m types.Map) (map[string]string, bool) {
	if m.IsNull() || m.IsUnknown() {
		return nil, false
	}
	// A null element is the only value ElementsAs cannot convert, and it fails
	// the whole map, leaving out empty: a dropped account_tags filter would
	// widen the scan to every account. Both map attributes reject null elements
	// at plan time (mapvalidator.NoNullValues), so this cannot fail here.
	out := map[string]string{}
	m.ElementsAs(context.Background(), &out, false)
	return out, true
}

type VPCTagInput struct {
	// (Required.)
	Key types.String `tfsdk:"key"`
	// (Required.)
	Value types.String `tfsdk:"value"`
}

type VPCConfigurationInput struct {
	// (Optional.)
	UseMondooVPC types.Bool `tfsdk:"use_mondoo_vpc"`
	// (Optional.)
	CIDR types.String `tfsdk:"cidr_block"`
	// (Optional.)
	VPCFlavour types.String `tfsdk:"vpc_flavour"`
	// (Optional.)
	VPCTag *VPCTagInput `tfsdk:"vpc_tag"`
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
	InstanceIdsFilter types.List `tfsdk:"instance_ids_filter"`
	// (Optional.)
	RegionsFilter types.List `tfsdk:"regions_filter"`
	// (Optional.)
	TagsFilter types.Map `tfsdk:"tags_filter"`
	// (Optional.)
	ExcludeInstanceIdsFilter types.List `tfsdk:"exclude_instance_ids_filter"`
	// (Optional.)
	ExcludeRegionsFilter types.List `tfsdk:"exclude_regions_filter"`
	// (Optional.)
	ExcludeTagsFilter types.Map `tfsdk:"exclude_tags_filter"`
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
			EventDetailType: mondoov1.String("AWS Console Sign-in via CloudTrail"),
		})
		eventScanTriggers = append(eventScanTriggers, &mondoov1.AWSEventPatternInput{
			ScanType:        mondoov1.String("ALL"),
			EventSource:     mondoov1.String("aws.ec2"),
			EventDetailType: mondoov1.String("EC2 Instance State-Change Notification"),
		})
	} else if m.ConsoleSignInTrigger.ValueBool() && !m.InstanceStateChangeTrigger.ValueBool() {
		eventScanTriggers = append(eventScanTriggers, &mondoov1.AWSEventPatternInput{
			ScanType:        mondoov1.String("ALL"),
			EventSource:     mondoov1.String("aws.signin"),
			EventDetailType: mondoov1.String("AWS Console Sign-in via CloudTrail"),
		})
	} else if m.InstanceStateChangeTrigger.ValueBool() && !m.ConsoleSignInTrigger.ValueBool() {
		eventScanTriggers = append(eventScanTriggers, &mondoov1.AWSEventPatternInput{
			ScanType:        mondoov1.String("ALL"),
			EventSource:     mondoov1.String("aws.ec2"),
			EventDetailType: mondoov1.String("EC2 Instance State-Change Notification"),
		})
	}

	var instanceIdsFilter []mondoov1.String
	instanceIds, _ := m.ScanConfiguration.Ec2ScanOptions.InstanceIdsFilter.ToListValue(context.Background())
	instanceIds.ElementsAs(context.Background(), &instanceIdsFilter, true)

	var regionsFilter []mondoov1.String
	regions, _ := m.ScanConfiguration.Ec2ScanOptions.RegionsFilter.ToListValue(context.Background())
	regions.ElementsAs(context.Background(), &regionsFilter, true)

	tagsFilter := make(mondoov1.Map)
	for k, v := range m.ScanConfiguration.Ec2ScanOptions.TagsFilter.Elements() {
		tagsFilter[k] = strings.Trim(v.String(), "\"")
	}

	var excludeInstanceIdsFilter []mondoov1.String
	excludeInstanceIds, _ := m.ScanConfiguration.Ec2ScanOptions.ExcludeInstanceIdsFilter.ToListValue(context.Background())
	excludeInstanceIds.ElementsAs(context.Background(), &excludeInstanceIdsFilter, true)

	var excludeRegionsFilter []mondoov1.String
	excludeRegions, _ := m.ScanConfiguration.Ec2ScanOptions.ExcludeRegionsFilter.ToListValue(context.Background())
	excludeRegions.ElementsAs(context.Background(), &excludeRegionsFilter, true)

	excludeTagsFilter := make(mondoov1.Map)
	for k, v := range m.ScanConfiguration.Ec2ScanOptions.ExcludeTagsFilter.Elements() {
		excludeTagsFilter[k] = strings.Trim(v.String(), "\"")
	}

	var accountIDs []mondoov1.String
	accountIds, _ := m.AccountIDs.ToListValue(context.Background())
	accountIds.ElementsAs(context.Background(), &accountIDs, true)
	opts = &mondoov1.AWSConfigurationOptionsInput{
		Region:         mondoov1.String(m.Region.ValueString()),
		IsOrganization: mondoov1.NewBooleanPtr(mondoov1.Boolean(m.IsOrganization.ValueBool())),
		AccountIDs:     &accountIDs,
		ScanConfiguration: mondoov1.ScanConfigurationInput{
			Ec2Scan:           mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.Ec2Scan.ValueBool())),
			EcrScan:           mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.EcrScan.ValueBool())),
			EcsScan:           mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.EcsScan.ValueBool())),
			CronScaninHours:   mondoov1.NewIntPtr(mondoov1.Int(m.ScanConfiguration.CronScaninHours.ValueInt64())),
			EventScanTriggers: &eventScanTriggers,
			Ec2ScanOptions: &mondoov1.Ec2ScanOptionsInput{
				Ssm:                       mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.Ec2ScanOptions.Ssm.ValueBool())),
				InstanceIDsFilter:         &instanceIdsFilter,
				RegionsFilter:             &regionsFilter,
				TagsFilter:                &tagsFilter,
				ExcludedInstanceIDsFilter: &excludeInstanceIdsFilter,
				ExcludedRegionsFilter:     &excludeRegionsFilter,
				ExcludedTagsFilter:        &excludeTagsFilter,
				EbsVolumeScan:             mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.Ec2ScanOptions.EbsVolumeScan.ValueBool())),
				InstanceConnect:           mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanConfiguration.Ec2ScanOptions.InstanceConnect.ValueBool())),
			},
		},
	}

	// account_scan is only sent when the practitioner set it. Leaving it off the
	// request lets the API apply its own default of true; sending false for an
	// unset attribute would silently disable account scanning for every
	// configuration that predates this attribute.
	if accountScan := m.ScanConfiguration.AccountScan; !accountScan.IsNull() && !accountScan.IsUnknown() {
		opts.ScanConfiguration.AccountScan = mondoov1.NewBooleanPtr(mondoov1.Boolean(accountScan.ValueBool()))
	}

	// The scan options below are only sent when set, so a configuration that
	// does not use them sends the same request as before they existed.
	opts.ScanConfiguration.CronScanExpression = OptionalString(m.ScanConfiguration.CronScanExpression)
	opts.ScanConfiguration.CrossAccountScan = OptionalBool(m.ScanConfiguration.CrossAccountScan)
	if m.ScanConfiguration.CrossAccountScanOptions != nil {
		opts.ScanConfiguration.CrossAccountScanOptions = m.ScanConfiguration.CrossAccountScanOptions.input()
	}
	opts.ScanConfiguration.PropagateAccountTags = OptionalBool(m.ScanConfiguration.PropagateAccountTags)
	if targets := m.ScanConfiguration.DiscoveryTargets; !targets.IsNull() && !targets.IsUnknown() {
		discoveryTargets := ConvertSliceStrings(targets)
		opts.ScanConfiguration.DiscoveryTargets = &discoveryTargets
	}
	if scheduling := m.ScanConfiguration.Scheduling; !scheduling.IsNull() && !scheduling.IsUnknown() {
		opts.ScanConfiguration.Scheduling = ToPtr(mondoov1.ScanScheduling(scheduling.ValueString()))
	}
	// WIF is only sent when set, for the same reason.
	opts.UseWif = OptionalBool(m.UseWif)
	opts.AwsAccountId = OptionalString(m.AwsAccountID)

	if m.ScanConfiguration.VpcConfiguration != nil {
		useMondooVPC := m.ScanConfiguration.VpcConfiguration.UseMondooVPC.ValueBool()

		var vpcFlavourPtr *mondoov1.VPCFlavour
		if m.ScanConfiguration.VpcConfiguration.VPCFlavour.ValueString() != "" {
			vpcFlavour := mondoov1.VPCFlavour(m.ScanConfiguration.VpcConfiguration.VPCFlavour.ValueString())
			vpcFlavourPtr = &vpcFlavour
		}

		var vpcTagPtr *mondoov1.VPCTagInput
		if vpcTag := m.ScanConfiguration.VpcConfiguration.VPCTag; vpcTag != nil &&
			vpcTag.Key.ValueString() != "" && vpcTag.Value.ValueString() != "" {
			vpcTagPtr = &mondoov1.VPCTagInput{
				Key:   mondoov1.String(vpcTag.Key.ValueString()),
				Value: mondoov1.String(vpcTag.Value.ValueString()),
			}
		}

		opts.ScanConfiguration.VpcConfiguration = &mondoov1.VPCConfigurationInput{
			UseMondooVPC:  mondoov1.NewBooleanPtr(mondoov1.Boolean(useMondooVPC)),
			UseDefaultVPC: mondoov1.NewBooleanPtr(mondoov1.Boolean(!useMondooVPC)),
			CIDR:          mondoov1.NewStringPtr(mondoov1.String(m.ScanConfiguration.VpcConfiguration.CIDR.ValueString())),
			VpcFlavour:    vpcFlavourPtr,
			VpcTag:        vpcTagPtr,
		}
	}

	return opts
}

func (r *integrationAwsServerlessResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_aws_serverless"
}

func (r *integrationAwsServerlessResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Continuously scan AWS organizations and accounts for misconfigurations and vulnerabilities.`,
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
				MarkdownDescription: "Integration identifier.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"token": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "Registration token for the integration. Pass it as the `MondooToken` parameter of the CloudFormation stack or StackSet; a separate `mondoo_registration_token` is not needed. With `use_wif`, pass `wif_config` as `MondooWifConfig` instead and leave `MondooToken` unset. The Mondoo Lambda exchanges it once per account, when the stack is created, for service account credentials that do not expire. The token expires 30 minutes after the integration is created, unless `is_organization` is `true`, in which case it never expires.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"source_bucket": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The S3 bucket the Lambda code is published to for the integration's region (populated by Mondoo after creation). Pass it as the `MondooSourceBucket` parameter of the `aws_cloudformation_stack` resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"cloud_formation_template_url": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The CloudFormation template URL for the integration's region (populated by Mondoo after creation). Use it as the `template_url` of the `aws_cloudformation_stack` resource.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"wif_config": schema.StringAttribute{
				Computed:            true,
				Sensitive:           true,
				MarkdownDescription: "For an integration with `use_wif`, how its Lambda authenticates to Mondoo (populated by Mondoo after creation). Pass it as the `MondooWifConfig` parameter of the `aws_cloudformation_stack` resource, in place of `MondooToken`. Unset for an integration without WIF.",
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
				MarkdownDescription: "Enable console sign-in trigger.",
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
						MarkdownDescription: "Enable AWS account scan. Defaults to true when not set.",
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
					"cron_scan_in_hours": schema.Int64Attribute{
						MarkdownDescription: "Cron scan in hours.",
						Optional:            true,
					},
					"cron_scan_expression": schema.StringAttribute{
						MarkdownDescription: "Scan schedule as a standard 5-field cron expression in UTC, e.g. `0 3 * * MON-FRI` for 03:00 on weekdays. Takes precedence over `cron_scan_in_hours`. Mondoo rejects an expression that restricts both day-of-month and day-of-week, or that scans more often than once an hour.",
						Optional:            true,
					},
					"cross_account_scan": schema.BoolAttribute{
						MarkdownDescription: "Enable cross-account scanning: the Lambda assumes `cross_account_scan_options.role_name` in each target account and scans it from the account it is deployed in.",
						Optional:            true,
					},
					"cross_account_scan_options": schema.SingleNestedAttribute{
						MarkdownDescription: "Which accounts cross-account scanning targets, and the IAM role it assumes in each.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"role_name": schema.StringAttribute{
								MarkdownDescription: "The IAM role the Lambda assumes in each target account. Set it when you create the integration. Once the deployed CloudFormation stack checks in, Mondoo keeps the role name the stack reports (its `CrossAccountScanRoleName` parameter), so a later change here is not applied: change it in the stack.",
								Optional:            true,
							},
							"account_ids": schema.ListAttribute{
								MarkdownDescription: "The AWS account IDs to scan.",
								Optional:            true,
								ElementType:         types.StringType,
							},
							"account_tags": schema.MapAttribute{
								MarkdownDescription: "Tag key to tag value. Only accounts that carry at least one of these tags are scanned; when empty, every account is scanned. A value of `*` matches any value of the key.",
								Optional:            true,
								ElementType:         types.StringType,
								Validators: []validator.Map{
									mapvalidator.NoNullValues(),
								},
							},
							"organizations": schema.MapAttribute{
								MarkdownDescription: "Foreign AWS Organizations to scan in addition to the Lambda's own organization: AWS Organization ID (e.g. `o-abc123defg`) to the 12-digit ID of its management (or delegated administrator) account. The Lambda assumes `role_name` in that account to list the organization's accounts, and scans every active account in it.",
								Optional:            true,
								ElementType:         types.StringType,
								Validators: []validator.Map{
									mapvalidator.NoNullValues(),
								},
							},
							"ebs_volume_scan": schema.BoolAttribute{
								MarkdownDescription: "Scan the target accounts' EC2 instances through snapshots of their EBS volumes. Takes effect only while `cross_account_scan` and `ec2_scan_options.ebs_volume_scan` are on, and only in target accounts whose Mondoo role was deployed with the `EbsScanning` opt-in.",
								Optional:            true,
							},
						},
					},
					"propagate_account_tags": schema.BoolAttribute{
						MarkdownDescription: "Propagate the AWS account tags to every asset discovered in the account.",
						Optional:            true,
					},
					"discovery_targets": schema.ListAttribute{
						MarkdownDescription: "The AWS discovery targets the account scan runs, e.g. `[\"auto\"]` or `[\"accounts\", \"s3-buckets\"]`. When unset, the Lambda uses its built-in targets. Targets that scan workloads directly, such as `instances` or `ecr`, are rejected: `ec2_scan` and `ecr_scan` cover those.",
						Optional:            true,
						ElementType:         types.StringType,
					},
					"scheduling": schema.StringAttribute{
						MarkdownDescription: "**Experimental:** this attribute may change or be removed in a future release. Who drives the scan cycle, one of: `LAMBDA` (the deployed Lambda keeps its own scan queue, the default) or `SERVER` (Mondoo keeps the queue, and the Lambda reports what it discovers and pulls its work). When unset, the integration keeps its current setting.",
						Optional:            true,
						Validators: []validator.String{
							stringvalidator.OneOf(string(mondoov1.ScanSchedulingLambda), string(mondoov1.ScanSchedulingServer)),
						},
					},
					"vpc_configuration": schema.SingleNestedAttribute{
						Optional: true,
						Attributes: map[string]schema.Attribute{
							"use_mondoo_vpc": schema.BoolAttribute{
								MarkdownDescription: "Use Mondoo VPC.",
								Optional:            true,
								DeprecationMessage:  "This field is deprecated and will be removed in the future.",
								Validators: []validator.Bool{
									boolvalidator.ConflictsWith(path.MatchRelative().AtParent().AtName("vpc_flavour")),
								},
							},
							"cidr_block": schema.StringAttribute{
								MarkdownDescription: "CIDR block for the Mondoo VPC.",
								Optional:            true,
							},
							"vpc_flavour": schema.StringAttribute{
								MarkdownDescription: "VPC flavour, one of: DEFAULT_VPC, MONDOO_NATGW, MONDOO_IGW, CUSTOM_VPC.",
								Optional:            true,
							},
							"vpc_tag": schema.SingleNestedAttribute{
								MarkdownDescription: "VPC tag to use when vpc_flavour is set to CUSTOM_VPC.",
								Optional:            true,
								Attributes: map[string]schema.Attribute{
									"key": schema.StringAttribute{
										MarkdownDescription: "Key of the VPC tag.",
										Required:            true,
									},
									"value": schema.StringAttribute{
										MarkdownDescription: "Value of the VPC tag.",
										Required:            true,
									},
								},
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
							"exclude_instance_ids_filter": schema.ListAttribute{
								MarkdownDescription: "List of instance IDs to exclude.",
								Optional:            true,
								ElementType:         types.StringType,
							},
							"exclude_regions_filter": schema.ListAttribute{
								MarkdownDescription: "List of regions to exclude.",
								Optional:            true,
								ElementType:         types.StringType,
							},
							"exclude_tags_filter": schema.MapAttribute{
								MarkdownDescription: "Excluded tags filter.",
								Optional:            true,
								ElementType:         types.StringType,
							},
							"ebs_volume_scan": schema.BoolAttribute{
								MarkdownDescription: "Enable EBS volume scan.",
								Optional:            true,
							},
							"ebs_scan_options": schema.SingleNestedAttribute{
								Optional:           true,
								DeprecationMessage: "This field is deprecated and will be removed in the future.",
								Attributes: map[string]schema.Attribute{
									"target_instances_per_scanner": schema.Int64Attribute{
										MarkdownDescription: "Target instances per scanner.",
										Optional:            true,
										DeprecationMessage:  "This field is deprecated and will be removed in the future.",
									},
									"max_asg_instances": schema.Int64Attribute{
										MarkdownDescription: "Max ASG instances.",
										Optional:            true,
										DeprecationMessage:  "This field is deprecated and will be removed in the future.",
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
				MarkdownDescription: "Set to `true` when you deploy the CloudFormation template with a StackSet, whether it targets the whole AWS organization or only some organizational units (OUs). Mondoo then issues a `token` that never expires, so accounts that join a targeted OU later can still register. When unset or `false`, the `token` expires 30 minutes after the integration is created. Set this when you create the integration: changing it later does not replace the `token` already in the Terraform state, so recreate the integration to get a token that does not expire. Cannot be combined with `account_ids`.",
				Optional:            true,
			},
			"use_wif": schema.BoolAttribute{
				MarkdownDescription: "Authenticate the Lambda to Mondoo with Workload Identity Federation instead of the `token`: Mondoo creates the WIF binding for the integration, and `wif_config` carries the stack's `MondooWifConfig` parameter. Requires `aws_account_id`. Immutable: changing it forces a new integration.",
				Optional:            true,
				PlanModifiers: []planmodifier.Bool{
					useWifRequiresReplace,
				},
			},
			"aws_account_id": schema.StringAttribute{
				MarkdownDescription: "The AWS account the Lambda is deployed in. Required when `use_wif` is true, and only valid then. Immutable: changing it forces a new integration.",
				Optional:            true,
				PlanModifiers: []planmodifier.String{
					awsAccountIDRequiresReplace,
				},
				Validators: []validator.String{
					// The WIF binding's subject: a typo makes an integration whose Lambda can never authenticate.
					stringvalidator.RegexMatches(regexp.MustCompile(`^\d{12}$`), "must be a 12-digit AWS account ID"),
				},
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

	resp.Diagnostics.Append(validateIntegrationAwsServerlessResourceModel(&data)...)
}

func validateIntegrationAwsServerlessResourceModel(data *integrationAwsServerlessResourceModel) (diagnostics diag.Diagnostics) {
	diagnostics.Append(validateIntegrationAwsServerlessWif(data)...)

	if data.ScanConfiguration.VpcConfiguration == nil {
		return
	}

	// user has provided mondoo vpc only
	if mondooVpc := data.ScanConfiguration.VpcConfiguration.UseMondooVPC.ValueBool(); mondooVpc {
		if cidr := data.ScanConfiguration.VpcConfiguration.CIDR.ValueString(); cidr == "" {
			diagnostics.AddError(
				"MissingAttributeError",
				"Attribute cidr_block must not be empty when use_mondoo_vpc is set to true.",
			)
		}
	}

	vpcFlavour := mondoov1.VPCFlavour(data.ScanConfiguration.VpcConfiguration.VPCFlavour.ValueString())
	if vpcFlavour != "" {
		allowedVpcFlavours := []mondoov1.VPCFlavour{
			mondoov1.VPCFlavourDefaultVpc, mondoov1.VPCFlavourMondooNatgw, mondoov1.VPCFlavourMondooIgw, mondoov1.VPCFlavourCustomVpc,
		}
		if !slices.Contains(allowedVpcFlavours, vpcFlavour) {
			diagnostics.AddError(
				"InvalidAttributeValueError",
				fmt.Sprintf("Attribute vpc_flavour must be one of %v, received: '%s'", allowedVpcFlavours, vpcFlavour),
			)
		}

		if cidr := data.ScanConfiguration.VpcConfiguration.CIDR.ValueString(); slices.Contains([]mondoov1.VPCFlavour{
			mondoov1.VPCFlavourMondooNatgw, mondoov1.VPCFlavourMondooIgw,
		}, vpcFlavour) && cidr == "" {
			diagnostics.AddError(
				"MissingAttributeError",
				"Attribute cidr_block must not be empty when Mondoo VPC is used.",
			)
		}

		if vpcFlavour == mondoov1.VPCFlavourCustomVpc {
			vpcTag := data.ScanConfiguration.VpcConfiguration.VPCTag
			if vpcTag == nil || vpcTag.Key.ValueString() == "" || vpcTag.Value.ValueString() == "" {
				diagnostics.AddError(
					"MissingAttributeError",
					"Attribute vpc_tag must not be empty when vpc_flavour is set to CUSTOM_VPC.",
				)
			}
		}
	}

	return
}

// useWifRequiresReplace replaces the integration when use_wif changes, as the
// API rejects the change. Unset and false are the same integration, so adding
// use_wif = false to one created without it updates it in place.
var useWifRequiresReplace = boolplanmodifier.RequiresReplaceIf(
	func(_ context.Context, req planmodifier.BoolRequest, resp *boolplanmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = req.PlanValue.IsUnknown() || req.PlanValue.ValueBool() != req.StateValue.ValueBool()
	},
	"Changing use_wif forces a new integration; unset and false are the same.",
	"Changing `use_wif` forces a new integration; unset and `false` are the same.",
)

// awsAccountIDRequiresReplace replaces the integration when aws_account_id
// changes, as the API rejects the change. Unset and empty are the same.
var awsAccountIDRequiresReplace = stringplanmodifier.RequiresReplaceIf(
	func(_ context.Context, req planmodifier.StringRequest, resp *stringplanmodifier.RequiresReplaceIfFuncResponse) {
		resp.RequiresReplace = req.PlanValue.IsUnknown() || req.PlanValue.ValueString() != req.StateValue.ValueString()
	},
	"Changing aws_account_id forces a new integration.",
	"Changing `aws_account_id` forces a new integration.",
)

// validateIntegrationAwsServerlessWif checks the WIF precondition the API
// enforces, so it fails at plan time rather than at apply: use_wif goes with
// aws_account_id. Values not known yet are skipped.
func validateIntegrationAwsServerlessWif(data *integrationAwsServerlessResourceModel) (diagnostics diag.Diagnostics) {
	if data.UseWif.IsUnknown() || data.AwsAccountID.IsUnknown() {
		return
	}
	useWif := data.UseWif.ValueBool()
	if useWif && data.AwsAccountID.ValueString() == "" {
		diagnostics.AddAttributeError(path.Root("aws_account_id"),
			"MissingAttributeError",
			"Attribute aws_account_id must not be empty when use_wif is set to true.",
		)
	}
	if !useWif && data.AwsAccountID.ValueString() != "" {
		diagnostics.AddAttributeError(path.Root("aws_account_id"),
			"ConflictingAttributesError",
			"Attribute aws_account_id is only valid when use_wif is set to true.",
		)
	}
	return
}

func (r *integrationAwsServerlessResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	// Prevent panic if the provider has not been configured.
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ExtendedGqlClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *ExtendedGqlClient. Got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)

		return
	}

	r.client = client
}

func (r *integrationAwsServerlessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationAwsServerlessResourceModel

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
	var accountIDs []mondoov1.String
	accountIds, _ := data.AccountIDs.ToListValue(context.Background())
	accountIds.ElementsAs(context.Background(), &accountIDs, true)

	// Check if both account IDs and is_organization are provided
	if len(accountIDs) > 0 && data.IsOrganization.ValueBool() {
		resp.Diagnostics.
			AddError("ConflictingAttributesError",
				"Cannot install CloudFormation stack to both AWS organization and accounts.",
			)
		return
	}

	tflog.Debug(ctx, "Creating integration")
	integration, err := r.client.CreateIntegration(ctx,
		space.MRN(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAws,
		mondoov1.ClientIntegrationConfigurationInput{
			AwsConfigurationOptions: data.GetConfigurationOptions(),
		})
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to create AWS integration. Got error: %s", err),
			)
		return
	}

	// Save space mrn into the Terraform state.
	data.Mrn = types.StringValue(string(integration.Mrn))
	data.Name = types.StringValue(string(integration.Name))
	data.Token = types.StringValue(string(integration.Token))
	data.SpaceID = types.StringValue(space.ID())

	// The create payload carries no configuration options, so read the
	// integration back for the server-computed CloudFormation inputs.
	r.refreshComputed(ctx, &data, "created", &resp.Diagnostics)

	// Save data into Terraform state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// refreshComputed reads the integration back and copies the server-computed
// CloudFormation inputs into the model. The mutation has already succeeded by
// the time this runs, so a failed read-back is a warning rather than an error:
// the next refresh picks the values up.
func (r *integrationAwsServerlessResource) refreshComputed(ctx context.Context, data *integrationAwsServerlessResourceModel, verb string, diags *diag.Diagnostics) {
	fetched, err := r.client.GetClientIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		diags.AddWarning(
			"Unable to read CloudFormation fields",
			fmt.Sprintf("The integration was %s, but source_bucket and cloud_formation_template_url "+
				"could not be read back: %s. Re-run 'terraform apply' to refresh them.", verb, err),
		)
		return
	}
	data.setComputedFrom(fetched)
}

// setComputedFrom copies the server-computed CloudFormation inputs into the model.
// wif_config cannot change once known, as use_wif cannot; it is only filled
// in. The server may hand back the link without it when it cannot reach the
// binding for a moment; that leaves wif_config as it was.
func (m *integrationAwsServerlessResourceModel) setComputedFrom(integration Integration) {
	opts := integration.ConfigurationOptions.AWSConfigurationOptions
	m.SourceBucket = types.StringValue(opts.SourceBucket)
	m.CloudFormationTemplateUrl = types.StringValue(opts.CloudFormationTemplateUrl)
	if !m.WifConfig.IsUnknown() && !m.WifConfig.IsNull() {
		return
	}
	if wifConfig := wifConfigFromQuickcreateUrl(opts.CloudFormationQuickcreateUrl); wifConfig != "" {
		m.WifConfig = types.StringValue(wifConfig)
	} else {
		m.WifConfig = types.StringNull()
	}
}

// wifConfigFromQuickcreateUrl returns the MondooWifConfig parameter of a
// CloudFormation quickcreate link, or "" when it carries none. The parameters
// are the query of the link's fragment, #/stacks/quickcreate?...; the value is
// base64, query-escaped.
func wifConfigFromQuickcreateUrl(quickcreateUrl string) string {
	u, err := url.Parse(quickcreateUrl)
	if err != nil {
		return ""
	}
	_, query, ok := strings.Cut(u.EscapedFragment(), "?")
	if !ok {
		return ""
	}
	params, err := url.ParseQuery(query)
	if err != nil {
		return ""
	}
	return params.Get("param_MondooWifConfig")
}

func (r *integrationAwsServerlessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationAwsServerlessResourceModel

	// Read Terraform prior state data into the model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// Refresh the server-computed CloudFormation inputs.
	if data.Mrn.ValueString() != "" {
		integration, err := r.client.GetClientIntegration(ctx, data.Mrn.ValueString())
		if err != nil {
			// Only drop the resource from state when it genuinely no longer
			// exists; a transient error must not make Terraform forget it.
			if isNotFoundError(err) {
				resp.State.RemoveResource(ctx)
				return
			}
			resp.Diagnostics.AddError("Client Error",
				fmt.Sprintf("Unable to read AWS serverless integration: %s", err),
			)
			return
		}
		data.setComputedFrom(integration)
	}

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

	// Check if both account IDs and is_organization are provided
	if len(accountIDs) > 0 && data.IsOrganization.ValueBool() {
		resp.Diagnostics.
			AddError("ConflictingAttributesError",
				"Cannot install CloudFormation stack to both AWS organization and accounts.",
			)
		return
	}

	if resp.Diagnostics.HasError() {
		return
	}

	// Do GraphQL request to API to update the resource.
	opts := mondoov1.ClientIntegrationConfigurationInput{
		AwsConfigurationOptions: data.GetConfigurationOptions(),
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeAws,
		opts,
	)
	if err != nil {
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf("Unable to update AWS integration. Got error: %s", err),
			)
		return
	}

	// The server recomputes the CloudFormation inputs from the region, so a
	// region change would otherwise leave stale values in state. wif_config
	// cannot change in an update; it stays as planned.
	wifConfig := data.WifConfig
	r.refreshComputed(ctx, &data, "updated", &resp.Diagnostics)
	data.WifConfig = wifConfig

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
		resp.Diagnostics.
			AddError("Client Error",
				fmt.Sprintf(
					"Unable to delete AWS serverless integration '%s'. Got error: %s",
					data.Mrn.ValueString(), err.Error(),
				),
			)
		return
	}
}

func (r *integrationAwsServerlessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	model := importedAwsServerlessModel(*integration)
	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// importedAwsServerlessModel is the state an import starts from: the
// integration's identity and space, and, for a WIF integration, use_wif and
// aws_account_id. Those two cannot change in place, so a WIF integration
// imported without them would plan a new integration on its first apply.
// Unset use_wif is the same as false, so it stays unset for the others.
func importedAwsServerlessModel(integration Integration) integrationAwsServerlessResourceModel {
	model := integrationAwsServerlessResourceModel{
		Mrn:     types.StringValue(integration.Mrn),
		Name:    types.StringValue(integration.Name),
		SpaceID: types.StringValue(integration.SpaceID()),
		// A list's zero value has no element type, which the state rejects.
		AccountIDs:        types.ListNull(types.StringType),
		ScanConfiguration: ScanConfigurationInput{DiscoveryTargets: types.ListNull(types.StringType)},
	}
	if opts := integration.ConfigurationOptions.AWSConfigurationOptions; opts.UseWif {
		model.UseWif = types.BoolValue(true)
		model.AwsAccountID = types.StringValue(opts.AwsAccountId)
	}
	return model
}
