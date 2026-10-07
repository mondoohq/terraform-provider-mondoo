// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int32validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	mondoov1 "go.mondoo.com/mondoo-go"
)

var (
	_ resource.Resource                = (*integrationKubernetesResource)(nil)
	_ resource.ResourceWithConfigure   = (*integrationKubernetesResource)(nil)
	_ resource.ResourceWithImportState = (*integrationKubernetesResource)(nil)
)

func NewIntegrationKubernetesResource() resource.Resource {
	return &integrationKubernetesResource{}
}

// integrationKubernetesResource manages a Kubernetes integration whose scan
// configuration the mondoo-operator pulls from Mondoo (remote-managed mode).
type integrationKubernetesResource struct {
	client *ExtendedGqlClient
}

type integrationKubernetesResourceModel struct {
	SpaceID types.String `tfsdk:"space_id"`
	Mrn     types.String `tfsdk:"mrn"`
	Name    types.String `tfsdk:"name"`

	ScanLocalCluster    types.Bool                   `tfsdk:"scan_local_cluster"`
	KubernetesResources *k8sKubernetesResourcesModel `tfsdk:"kubernetes_resources"`
	Nodes               *k8sNodesModel               `tfsdk:"nodes"`
	Containers          *k8sContainersModel          `tfsdk:"containers"`
	Scanner             *k8sScannerModel             `tfsdk:"scanner"`
	Namespaces          *k8sIncludeExcludeModel      `tfsdk:"namespaces"`
	JobOverrides        *k8sJobOverridesModel        `tfsdk:"job_overrides"`
	AssetAnnotations    types.Map                    `tfsdk:"asset_annotations"`
}

type k8sKubernetesResourcesModel struct {
	Enable                types.Bool               `tfsdk:"enable"`
	Schedule              types.String             `tfsdk:"schedule"`
	ActiveDeadlineSeconds types.Int32              `tfsdk:"active_deadline_seconds"`
	JobOverrides          *k8sJobOverridesModel    `tfsdk:"job_overrides"`
	ResourceWatcher       *k8sResourceWatcherModel `tfsdk:"resource_watcher"`
}

type k8sNodesModel struct {
	Enable            types.Bool            `tfsdk:"enable"`
	Style             types.String          `tfsdk:"style"`
	Schedule          types.String          `tfsdk:"schedule"`
	IntervalTimer     types.Int32           `tfsdk:"interval_timer"`
	PriorityClassName types.String          `tfsdk:"priority_class_name"`
	Resources         *k8sResourcesModel    `tfsdk:"resources"`
	JobOverrides      *k8sJobOverridesModel `tfsdk:"job_overrides"`
	Env               []k8sEnvVarModel      `tfsdk:"env"`
}

type k8sContainersModel struct {
	Enable                types.Bool              `tfsdk:"enable"`
	Schedule              types.String            `tfsdk:"schedule"`
	ActiveDeadlineSeconds types.Int32             `tfsdk:"active_deadline_seconds"`
	ScanCache             *k8sScanCacheModel      `tfsdk:"scan_cache"`
	Repositories          *k8sIncludeExcludeModel `tfsdk:"repositories"`
	Resources             *k8sResourcesModel      `tfsdk:"resources"`
	JobOverrides          *k8sJobOverridesModel   `tfsdk:"job_overrides"`
	Env                   []k8sEnvVarModel        `tfsdk:"env"`
}

type k8sScannerModel struct {
	Replicas                        types.Int32        `tfsdk:"replicas"`
	PrivateRegistriesPullSecretRefs types.List         `tfsdk:"private_registries_pull_secret_refs"`
	Resources                       *k8sResourcesModel `tfsdk:"resources"`
	Env                             []k8sEnvVarModel   `tfsdk:"env"`
}

type k8sIncludeExcludeModel struct {
	Include types.List `tfsdk:"include"`
	Exclude types.List `tfsdk:"exclude"`
}

type k8sScanCacheModel struct {
	Enable types.Bool   `tfsdk:"enable"`
	TTL    types.String `tfsdk:"ttl"`
}

type k8sResourceWatcherModel struct {
	Enable              types.Bool   `tfsdk:"enable"`
	WatchAllResources   types.Bool   `tfsdk:"watch_all_resources"`
	ResourceTypes       types.List   `tfsdk:"resource_types"`
	DebounceInterval    types.String `tfsdk:"debounce_interval"`
	MinimumScanInterval types.String `tfsdk:"minimum_scan_interval"`
}

type k8sResourcesModel struct {
	Requests *k8sResourceQuantitiesModel `tfsdk:"requests"`
	Limits   *k8sResourceQuantitiesModel `tfsdk:"limits"`
}

type k8sResourceQuantitiesModel struct {
	CPU    types.String `tfsdk:"cpu"`
	Memory types.String `tfsdk:"memory"`
}

type k8sJobOverridesModel struct {
	TTLSecondsAfterFinished types.Int32          `tfsdk:"ttl_seconds_after_finished"`
	Annotations             types.Map            `tfsdk:"annotations"`
	Labels                  types.Map            `tfsdk:"labels"`
	NodeSelector            types.Map            `tfsdk:"node_selector"`
	Tolerations             []k8sTolerationModel `tfsdk:"tolerations"`
}

type k8sTolerationModel struct {
	Key      types.String `tfsdk:"key"`
	Operator types.String `tfsdk:"operator"`
	Value    types.String `tfsdk:"value"`
	Effect   types.String `tfsdk:"effect"`
}

type k8sEnvVarModel struct {
	Name  types.String `tfsdk:"name"`
	Value types.String `tfsdk:"value"`
}

// k8sNodeScanStyles maps the node scan styles accepted in Terraform to the API's
// enum values. The API also knows DEPLOYMENT, but it reports it back as
// DAEMONSET, so it can't round-trip.
var k8sNodeScanStyles = map[string]mondoov1.K8sScanNodesStyle{
	"cronjob":   mondoov1.K8sScanNodesStyleCronjob,
	"daemonset": mondoov1.K8sScanNodesStyleDaemonset,
}

// k8sDurationRegex matches the Go durations the operator parses, such as "8760h"
// or "1m30s".
var k8sDurationRegex = regexp.MustCompile(`^([0-9]+(\.[0-9]+)?(ns|us|µs|ms|s|m|h))+$`)

func (r *integrationKubernetesResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_integration_kubernetes"
}

func k8sDurationAttribute(description string) schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: description,
		Optional:            true,
		Validators: []validator.String{
			stringvalidator.RegexMatches(k8sDurationRegex, `must be a duration such as "30s", "5m" or "8760h"`),
		},
	}
}

func k8sStringListAttribute(description string) schema.ListAttribute {
	return schema.ListAttribute{
		MarkdownDescription: description,
		Optional:            true,
		ElementType:         types.StringType,
	}
}

func k8sStringMapAttribute(description string) schema.MapAttribute {
	return schema.MapAttribute{
		MarkdownDescription: description,
		Optional:            true,
		ElementType:         types.StringType,
	}
}

func k8sScheduleAttribute(what string) schema.StringAttribute {
	return schema.StringAttribute{
		MarkdownDescription: fmt.Sprintf("Cron schedule for %s, such as `0 */6 * * *`. If unset, the operator picks a minute of every hour based on the cluster.", what),
		Optional:            true,
		Validators: []validator.String{
			stringvalidator.LengthAtLeast(1),
		},
	}
}

func k8sResourcesAttribute(what string) schema.SingleNestedAttribute {
	quantities := func(kind string) schema.SingleNestedAttribute {
		return schema.SingleNestedAttribute{
			MarkdownDescription: fmt.Sprintf("Resource %s, as Kubernetes quantities.", kind),
			Optional:            true,
			Attributes: map[string]schema.Attribute{
				"cpu": schema.StringAttribute{
					MarkdownDescription: "CPU, such as `100m`.",
					Optional:            true,
				},
				"memory": schema.StringAttribute{
					MarkdownDescription: "Memory, such as `256Mi`.",
					Optional:            true,
				},
			},
		}
	}
	return schema.SingleNestedAttribute{
		MarkdownDescription: fmt.Sprintf("Compute resources for %s.", what),
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"requests": quantities("requests"),
			"limits":   quantities("limits"),
		},
	}
}

func k8sJobOverridesAttribute(description string) schema.SingleNestedAttribute {
	return schema.SingleNestedAttribute{
		MarkdownDescription: description,
		Optional:            true,
		Attributes: map[string]schema.Attribute{
			"ttl_seconds_after_finished": schema.Int32Attribute{
				MarkdownDescription: "Seconds after which finished scan Jobs are deleted.",
				Optional:            true,
				Validators: []validator.Int32{
					int32validator.AtLeast(0),
				},
			},
			"annotations":   k8sStringMapAttribute("Annotations added to the scan Jobs and their pods, such as `karpenter.sh/do-not-disrupt`."),
			"labels":        k8sStringMapAttribute("Labels added to the scan Jobs and their pods."),
			"node_selector": k8sStringMapAttribute("Node selector for the scan pods."),
			"tolerations": schema.ListNestedAttribute{
				MarkdownDescription: "Tolerations for the scan pods.",
				Optional:            true,
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"key": schema.StringAttribute{
							MarkdownDescription: "Taint key the toleration applies to. Leave it unset with operator `Exists` to tolerate every taint.",
							Optional:            true,
						},
						"operator": schema.StringAttribute{
							MarkdownDescription: "`Equal` or `Exists`. Kubernetes defaults to `Equal`.",
							Optional:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("Equal", "Exists"),
							},
						},
						"value": schema.StringAttribute{
							MarkdownDescription: "Taint value the toleration matches, with operator `Equal`.",
							Optional:            true,
						},
						"effect": schema.StringAttribute{
							MarkdownDescription: "`NoSchedule`, `PreferNoSchedule` or `NoExecute`. Leave it unset to match every effect.",
							Optional:            true,
							Validators: []validator.String{
								stringvalidator.OneOf("NoSchedule", "PreferNoSchedule", "NoExecute"),
							},
						},
					},
				},
			},
		},
	}
}

func k8sEnvAttribute(what string) schema.ListNestedAttribute {
	return schema.ListNestedAttribute{
		MarkdownDescription: fmt.Sprintf("Environment variables for %s.", what),
		Optional:            true,
		NestedObject: schema.NestedAttributeObject{
			Attributes: map[string]schema.Attribute{
				"name": schema.StringAttribute{
					MarkdownDescription: "Name of the variable.",
					Required:            true,
					Validators: []validator.String{
						stringvalidator.LengthAtLeast(1),
					},
				},
				"value": schema.StringAttribute{
					MarkdownDescription: "Value of the variable.",
					Optional:            true,
				},
			},
		},
	}
}

func k8sActiveDeadlineAttribute(what string) schema.Int32Attribute {
	return schema.Int32Attribute{
		MarkdownDescription: fmt.Sprintf("Seconds after which a running %s is stopped.", what),
		Optional:            true,
		Validators: []validator.Int32{
			int32validator.AtLeast(1),
		},
	}
}

func (r *integrationKubernetesResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `Manages a Kubernetes integration and the scan configuration of the mondoo-operator that reports to it.

~> **This resource only works together with a remote-managed operator.** The integration holds the scan configuration, and the mondoo-operator (v13.4.0 or later) applies it only when its credentials Secret holds this resource's ` + "`mrn`" + ` under the ` + "`integrationmrn`" + ` key and its MondooAuditConfig sets ` + "`spec.remoteManaged: true`" + `, ` + "`spec.consoleIntegration.enable: true`" + ` and ` + "`spec.consoleIntegration.autoCreate: false`" + `. Without that setup, the operator scans with its own spec and the integration shows a configuration that doesn't run in the cluster.`,
		Attributes: map[string]schema.Attribute{
			"space_id": schema.StringAttribute{
				MarkdownDescription: "Mondoo space identifier. If there is no space ID, the provider space is used.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
					stringplanmodifier.RequiresReplace(),
				},
			},
			"mrn": schema.StringAttribute{
				MarkdownDescription: "Integration identifier. Store it in the `integrationmrn` key of the operator's credentials Secret.",
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
			"scan_local_cluster": schema.BoolAttribute{
				MarkdownDescription: "Whether the operator scans the cluster it runs in. When `false`, the operator scans neither the cluster's Kubernetes resources nor its nodes. Defaults to `true`.",
				Optional:            true,
				Computed:            true,
				Default:             booldefault.StaticBool(true),
			},
			"kubernetes_resources": schema.SingleNestedAttribute{
				MarkdownDescription: "Scanning of the cluster's Kubernetes resources, such as Deployments, Pods and Ingresses. Disabled if unset.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"enable": schema.BoolAttribute{
						MarkdownDescription: "Whether to scan the cluster's Kubernetes resources.",
						Required:            true,
					},
					"schedule":                k8sScheduleAttribute("Kubernetes resource scans"),
					"active_deadline_seconds": k8sActiveDeadlineAttribute("Kubernetes resource scan"),
					"job_overrides":           k8sJobOverridesAttribute("Job settings for Kubernetes resource scans. They take precedence over the top-level `job_overrides`."),
					"resource_watcher": schema.SingleNestedAttribute{
						MarkdownDescription: "Scans of Kubernetes resources as soon as they change, in addition to the scheduled scans.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"enable": schema.BoolAttribute{
								MarkdownDescription: "Whether to watch Kubernetes resources for changes.",
								Optional:            true,
							},
							"watch_all_resources": schema.BoolAttribute{
								MarkdownDescription: "Whether to watch all resource types, including short-lived ones such as Pods, Jobs and CronJobs. By default, only Deployments, DaemonSets, StatefulSets and ReplicaSets are watched.",
								Optional:            true,
							},
							"resource_types":        k8sStringListAttribute("Resource types to watch, such as `deployments` or `pods`. If unset, the types follow `watch_all_resources`."),
							"debounce_interval":     k8sDurationAttribute("How long to collect changes before scanning them, such as `10s`. The operator defaults to 10 seconds."),
							"minimum_scan_interval": k8sDurationAttribute("Minimum time between two scans, such as `2m`. The operator defaults to 2 minutes."),
						},
					},
				},
			},
			"nodes": schema.SingleNestedAttribute{
				MarkdownDescription: "Scanning of the cluster's nodes. Disabled if unset.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"enable": schema.BoolAttribute{
						MarkdownDescription: "Whether to scan the cluster's nodes.",
						Required:            true,
					},
					"style": schema.StringAttribute{
						MarkdownDescription: "How the operator runs node scans: `cronjob` (the default, one Job per node on `schedule`) or `daemonset` (one pod on every node, scanning every `interval_timer` minutes).",
						Optional:            true,
						Validators: []validator.String{
							stringvalidator.OneOf("cronjob", "daemonset"),
						},
					},
					"schedule": k8sScheduleAttribute("node scans with the `cronjob` style"),
					"interval_timer": schema.Int32Attribute{
						MarkdownDescription: "Minutes between two node scans with the `daemonset` style. The operator defaults to 60.",
						Optional:            true,
						Validators: []validator.Int32{
							int32validator.AtLeast(1),
						},
					},
					"priority_class_name": schema.StringAttribute{
						MarkdownDescription: "Name of the PriorityClass of the node scan pods. The PriorityClass must exist in the cluster.",
						Optional:            true,
					},
					"resources":     k8sResourcesAttribute("node scans"),
					"job_overrides": k8sJobOverridesAttribute("Job and pod settings for node scans. They take precedence over the top-level `job_overrides`."),
					"env":           k8sEnvAttribute("node scans"),
				},
			},
			"containers": schema.SingleNestedAttribute{
				MarkdownDescription: "Scanning of the container images that run in the cluster. Disabled if unset.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"enable": schema.BoolAttribute{
						MarkdownDescription: "Whether to scan the container images that run in the cluster.",
						Required:            true,
					},
					"schedule":                k8sScheduleAttribute("container image scans"),
					"active_deadline_seconds": k8sActiveDeadlineAttribute("container image scan"),
					"scan_cache": schema.SingleNestedAttribute{
						MarkdownDescription: "Caching of image scan results, so that an image is scanned again only after the cache entry expires.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"enable": schema.BoolAttribute{
								MarkdownDescription: "Whether to cache image scan results.",
								Required:            true,
							},
							"ttl": k8sDurationAttribute("How long a cached result stays valid, such as `24h`."),
						},
					},
					"repositories": schema.SingleNestedAttribute{
						MarkdownDescription: "Container image repositories to scan or skip.",
						Optional:            true,
						Attributes: map[string]schema.Attribute{
							"include": k8sStringListAttribute("Repositories to scan. If set, images from other repositories are skipped."),
							"exclude": k8sStringListAttribute("Repositories to skip."),
						},
					},
					"resources":     k8sResourcesAttribute("container image scans"),
					"job_overrides": k8sJobOverridesAttribute("Job and pod settings for container image scans. They take precedence over the top-level `job_overrides`."),
					"env":           k8sEnvAttribute("container image scans"),
				},
			},
			"scanner": schema.SingleNestedAttribute{
				MarkdownDescription: "Settings of the scanner that runs Kubernetes resource scans.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"replicas": schema.Int32Attribute{
						MarkdownDescription: "Number of scanner replicas. The operator defaults to 1.",
						Optional:            true,
						Validators: []validator.Int32{
							int32validator.AtLeast(1),
						},
					},
					"private_registries_pull_secret_refs": k8sStringListAttribute("Names of Secrets, in the operator's namespace, that hold credentials for private container registries."),
					"resources":                           k8sResourcesAttribute("the scanner"),
					"env":                                 k8sEnvAttribute("the scanner"),
				},
			},
			"namespaces": schema.SingleNestedAttribute{
				MarkdownDescription: "Namespaces to scan or skip.",
				Optional:            true,
				Attributes: map[string]schema.Attribute{
					"include": k8sStringListAttribute("Namespaces to scan. If set, all other namespaces are skipped."),
					"exclude": k8sStringListAttribute("Namespaces to skip."),
				},
			},
			"job_overrides":     k8sJobOverridesAttribute("Job and pod settings for all scan types. The per-type `job_overrides` take precedence."),
			"asset_annotations": k8sStringMapAttribute("Annotations added to every asset the operator scans."),
		},
	}
}

func (r *integrationKubernetesResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *integrationKubernetesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data integrationKubernetesResourceModel
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

	opts, diags := data.configurationInput(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Kubernetes integration")
	integration, err := r.client.CreateIntegration(ctx,
		space.MRN(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeK8s,
		opts,
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to create Kubernetes integration. Got error: %s", err),
		)
		return
	}

	data.Mrn = types.StringValue(string(integration.Mrn))
	data.SpaceID = types.StringValue(space.ID())

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationKubernetesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data integrationKubernetesResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	integration, err := r.client.getKubernetesIntegration(ctx, data.Mrn.ValueString())
	if err != nil {
		// Only drop the resource from state when it no longer exists. A transient
		// error must not make Terraform forget the integration.
		if isNotFoundError(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to read Kubernetes integration. Got error: %s", err),
		)
		return
	}

	model, diags := integration.model(ctx, &data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

func (r *integrationKubernetesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data integrationKubernetesResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	opts, diags := data.configurationInput(ctx)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.UpdateIntegration(ctx,
		data.Mrn.ValueString(),
		data.Name.ValueString(),
		mondoov1.ClientIntegrationTypeK8s,
		opts,
	)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to update Kubernetes integration. Got error: %s", err),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *integrationKubernetesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data integrationKubernetesResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.DeleteIntegration(ctx, data.Mrn.ValueString())
	if err != nil && !isNotFoundError(err) {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to delete Kubernetes integration. Got error: %s", err),
		)
	}
}

func (r *integrationKubernetesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	integration, ok := r.client.ImportIntegration(ctx, req, resp)
	if !ok {
		return
	}

	k8s, err := r.client.getKubernetesIntegration(ctx, integration.Mrn)
	if err != nil {
		resp.Diagnostics.AddError("Client Error",
			fmt.Sprintf("Unable to read Kubernetes integration. Got error: %s", err),
		)
		return
	}

	// With no prior state, every unset setting is imported as null, which matches
	// a configuration that leaves it out.
	model, diags := k8s.model(ctx, nil)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &model)...)
}

// configurationInput converts the model into the API's configuration input.
// Unset settings are left out, which clears them on the integration.
func (m integrationKubernetesResourceModel) configurationInput(ctx context.Context) (mondoov1.ClientIntegrationConfigurationInput, diag.Diagnostics) {
	var diags diag.Diagnostics
	opts := &mondoov1.K8sConfigurationOptionsInput{
		ScanLocalCluster: mondoov1.NewBooleanPtr(mondoov1.Boolean(m.ScanLocalCluster.ValueBool())),
		// Admission control was removed from the operator in v12.1.0.
		ScanDeploys: false,
	}

	if kr := m.KubernetesResources; kr != nil {
		opts.ScanWorkloads = mondoov1.Boolean(kr.Enable.ValueBool())
		opts.Schedule = k8sStringInput(kr.Schedule)
		opts.K8sActiveDeadline = k8sIntInput(kr.ActiveDeadlineSeconds)
		opts.ScannerJobOverrides = k8sJobOverridesInput(ctx, kr.JobOverrides, &diags)
		if rw := kr.ResourceWatcher; rw != nil {
			opts.ResourceWatcher = &mondoov1.K8sResourceWatcherInput{
				Enable:              k8sBoolInput(rw.Enable),
				WatchAllResources:   k8sBoolInput(rw.WatchAllResources),
				ResourceTypes:       k8sStringListInput(ctx, rw.ResourceTypes, &diags),
				DebounceInterval:    k8sStringInput(rw.DebounceInterval),
				MinimumScanInterval: k8sStringInput(rw.MinimumScanInterval),
			}
		}
	}

	if n := m.Nodes; n != nil {
		opts.ScanNodes = mondoov1.Boolean(n.Enable.ValueBool())
		if k8sKnown(n.Style) {
			// The schema validator only accepts the styles in k8sNodeScanStyles, so
			// a miss here means the two have drifted apart.
			style, ok := k8sNodeScanStyles[n.Style.ValueString()]
			if !ok {
				diags.AddAttributeError(path.Root("nodes").AtName("style"),
					"Unsupported node scan style",
					fmt.Sprintf("The node scan style %q has no API value. Please report this issue to the provider developers.", n.Style.ValueString()),
				)
			} else {
				opts.ScanNodesStyle = &style
			}
		}
		opts.NodesSchedule = k8sStringInput(n.Schedule)
		opts.NodesIntervalTimer = k8sIntInput(n.IntervalTimer)
		opts.NodesPriorityClassName = k8sStringInput(n.PriorityClassName)
		opts.NodesResources = k8sResourcesInput(n.Resources)
		opts.NodesJobOverrides = k8sJobOverridesInput(ctx, n.JobOverrides, &diags)
		opts.NodesEnv = k8sEnvInput(n.Env)
	}

	if c := m.Containers; c != nil {
		opts.ScanPublicImages = mondoov1.NewBooleanPtr(mondoov1.Boolean(c.Enable.ValueBool()))
		opts.ContainersSchedule = k8sStringInput(c.Schedule)
		opts.ContainersActiveDeadline = k8sIntInput(c.ActiveDeadlineSeconds)
		if sc := c.ScanCache; sc != nil {
			opts.ScanCacheEnabled = mondoov1.NewBooleanPtr(mondoov1.Boolean(sc.Enable.ValueBool()))
			opts.ScanCacheTtl = k8sStringInput(sc.TTL)
		}
		if repos := c.Repositories; repos != nil {
			opts.ContainerRepositoriesAllowList = k8sStringListInput(ctx, repos.Include, &diags)
			opts.ContainerRepositoriesDenyList = k8sStringListInput(ctx, repos.Exclude, &diags)
		}
		opts.ContainersResources = k8sResourcesInput(c.Resources)
		opts.ContainersJobOverrides = k8sJobOverridesInput(ctx, c.JobOverrides, &diags)
		opts.ContainersEnv = k8sEnvInput(c.Env)
	} else {
		opts.ScanPublicImages = mondoov1.NewBooleanPtr(false)
	}

	if s := m.Scanner; s != nil {
		opts.ScannerReplicas = k8sIntInput(s.Replicas)
		opts.PrivateRegistriesPullSecretRefs = k8sStringListInput(ctx, s.PrivateRegistriesPullSecretRefs, &diags)
		opts.ScannerResources = k8sResourcesInput(s.Resources)
		opts.ScannerEnv = k8sEnvInput(s.Env)
	}

	if ns := m.Namespaces; ns != nil {
		opts.NamespaceAllowList = k8sStringListInput(ctx, ns.Include, &diags)
		opts.NamespaceDenyList = k8sStringListInput(ctx, ns.Exclude, &diags)
	}

	opts.JobOverrides = k8sJobOverridesInput(ctx, m.JobOverrides, &diags)
	opts.AssetAnnotations = k8sMapInput(ctx, m.AssetAnnotations, &diags)

	return mondoov1.ClientIntegrationConfigurationInput{K8sConfigurationOptions: opts}, diags
}

func k8sStringInput(v types.String) *mondoov1.String {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return mondoov1.NewStringPtr(mondoov1.String(v.ValueString()))
}

func k8sIntInput(v types.Int32) *mondoov1.Int {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return mondoov1.NewIntPtr(mondoov1.Int(v.ValueInt32()))
}

func k8sBoolInput(v types.Bool) *mondoov1.Boolean {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return mondoov1.NewBooleanPtr(mondoov1.Boolean(v.ValueBool()))
}

func k8sStringListInput(ctx context.Context, v types.List, diags *diag.Diagnostics) *[]mondoov1.String {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	var values []string
	diags.Append(v.ElementsAs(ctx, &values, false)...)
	list := make([]mondoov1.String, 0, len(values))
	for _, value := range values {
		list = append(list, mondoov1.String(value))
	}
	return &list
}

func k8sMapInput(ctx context.Context, v types.Map, diags *diag.Diagnostics) *mondoov1.Map {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	var values map[string]string
	diags.Append(v.ElementsAs(ctx, &values, false)...)
	m := make(mondoov1.Map, len(values))
	for k, value := range values {
		m[k] = value
	}
	return &m
}

func k8sResourcesInput(r *k8sResourcesModel) *mondoov1.K8sResourceRequirementsInput {
	if r == nil {
		return nil
	}
	input := &mondoov1.K8sResourceRequirementsInput{}
	if r.Requests != nil {
		input.CpuRequest = k8sStringInput(r.Requests.CPU)
		input.MemRequest = k8sStringInput(r.Requests.Memory)
	}
	if r.Limits != nil {
		input.CpuLimit = k8sStringInput(r.Limits.CPU)
		input.MemLimit = k8sStringInput(r.Limits.Memory)
	}
	return input
}

func k8sJobOverridesInput(ctx context.Context, jo *k8sJobOverridesModel, diags *diag.Diagnostics) *mondoov1.K8sJobOverridesInput {
	if jo == nil {
		return nil
	}
	input := &mondoov1.K8sJobOverridesInput{
		TtlSecondsAfterFinished: k8sIntInput(jo.TTLSecondsAfterFinished),
		Annotations:             k8sMapInput(ctx, jo.Annotations, diags),
		Labels:                  k8sMapInput(ctx, jo.Labels, diags),
		NodeSelector:            k8sMapInput(ctx, jo.NodeSelector, diags),
	}
	if jo.Tolerations != nil {
		tolerations := make([]mondoov1.K8sTolerationInput, 0, len(jo.Tolerations))
		for _, t := range jo.Tolerations {
			tolerations = append(tolerations, mondoov1.K8sTolerationInput{
				Key:      k8sStringInput(t.Key),
				Operator: k8sStringInput(t.Operator),
				Value:    k8sStringInput(t.Value),
				Effect:   k8sStringInput(t.Effect),
			})
		}
		input.Tolerations = &tolerations
	}
	return input
}

func k8sEnvInput(env []k8sEnvVarModel) *[]mondoov1.K8sEnvVarInput {
	if env == nil {
		return nil
	}
	vars := make([]mondoov1.K8sEnvVarInput, 0, len(env))
	for _, e := range env {
		vars = append(vars, mondoov1.K8sEnvVarInput{
			Name:  mondoov1.String(e.Name.ValueString()),
			Value: k8sStringInput(e.Value),
		})
	}
	return &vars
}

// k8sIntegration is a Kubernetes integration as the API returns it.
type k8sIntegration struct {
	Mrn                  string                     `json:"mrn"`
	Name                 string                     `json:"name"`
	Type                 string                     `json:"type"`
	ConfigurationOptions k8sConfigurationOptionsAPI `json:"configurationOptions"`
}

type k8sConfigurationOptionsAPI struct {
	ScanNodes                       *bool                       `json:"scanNodes"`
	ScanNodesStyle                  string                      `json:"scanNodesStyle"`
	ScanWorkloads                   *bool                       `json:"scanWorkloads"`
	ScanPublicImages                *bool                       `json:"scanPublicImages"`
	ScanLocalCluster                *bool                       `json:"scanLocalCluster"`
	NamespaceAllowList              []string                    `json:"namespaceAllowList"`
	NamespaceDenyList               []string                    `json:"namespaceDenyList"`
	PrivateRegistriesPullSecretRefs []string                    `json:"privateRegistriesPullSecretRefs"`
	Schedule                        *string                     `json:"schedule"`
	NodesSchedule                   *string                     `json:"nodesSchedule"`
	ContainersSchedule              *string                     `json:"containersSchedule"`
	ScannerReplicas                 *int32                      `json:"scannerReplicas"`
	ScannerResources                *k8sResourceRequirementsAPI `json:"scannerResources"`
	NodesResources                  *k8sResourceRequirementsAPI `json:"nodesResources"`
	ContainersResources             *k8sResourceRequirementsAPI `json:"containersResources"`
	ResourceWatcher                 *k8sResourceWatcherAPI      `json:"resourceWatcher"`
	ContainerRepositoriesAllowList  []string                    `json:"containerRepositoriesAllowList"`
	ContainerRepositoriesDenyList   []string                    `json:"containerRepositoriesDenyList"`
	ScanCacheEnabled                *bool                       `json:"scanCacheEnabled"`
	ScanCacheTTL                    *string                     `json:"scanCacheTtl"`
	K8sActiveDeadline               *int32                      `json:"k8sActiveDeadline"`
	ContainersActiveDeadline        *int32                      `json:"containersActiveDeadline"`
	JobOverrides                    *k8sJobOverridesAPI         `json:"jobOverrides"`
	ScannerJobOverrides             *k8sJobOverridesAPI         `json:"scannerJobOverrides"`
	NodesJobOverrides               *k8sJobOverridesAPI         `json:"nodesJobOverrides"`
	ContainersJobOverrides          *k8sJobOverridesAPI         `json:"containersJobOverrides"`
	AssetAnnotations                map[string]string           `json:"assetAnnotations"`
	NodesPriorityClassName          *string                     `json:"nodesPriorityClassName"`
	NodesIntervalTimer              *int32                      `json:"nodesIntervalTimer"`
	ScannerEnv                      []k8sEnvVarAPI              `json:"scannerEnv"`
	NodesEnv                        []k8sEnvVarAPI              `json:"nodesEnv"`
	ContainersEnv                   []k8sEnvVarAPI              `json:"containersEnv"`
}

type k8sResourceRequirementsAPI struct {
	CPURequest *string `json:"cpuRequest"`
	CPULimit   *string `json:"cpuLimit"`
	MemRequest *string `json:"memRequest"`
	MemLimit   *string `json:"memLimit"`
}

type k8sResourceWatcherAPI struct {
	Enable              *bool    `json:"enable"`
	DebounceInterval    *string  `json:"debounceInterval"`
	MinimumScanInterval *string  `json:"minimumScanInterval"`
	WatchAllResources   *bool    `json:"watchAllResources"`
	ResourceTypes       []string `json:"resourceTypes"`
}

type k8sJobOverridesAPI struct {
	TTLSecondsAfterFinished *int32             `json:"ttlSecondsAfterFinished"`
	Annotations             map[string]string  `json:"annotations"`
	NodeSelector            map[string]string  `json:"nodeSelector"`
	Labels                  map[string]string  `json:"labels"`
	Tolerations             []k8sTolerationAPI `json:"tolerations"`
}

type k8sTolerationAPI struct {
	Key      *string `json:"key"`
	Operator *string `json:"operator"`
	Value    *string `json:"value"`
	Effect   *string `json:"effect"`
}

type k8sEnvVarAPI struct {
	Name  string  `json:"name"`
	Value *string `json:"value"`
}

const k8sIntegrationQuery = `
query KubernetesIntegration($mrn: String!) {
  clientIntegration(input: {mrn: $mrn}) {
    integration {
      mrn
      name
      type
      configurationOptions {
        ... on K8sConfigurationOptions {
          scanNodes
          scanNodesStyle
          scanWorkloads
          scanPublicImages
          scanLocalCluster
          namespaceAllowList
          namespaceDenyList
          privateRegistriesPullSecretRefs
          schedule
          nodesSchedule
          containersSchedule
          scannerReplicas
          scannerResources { ...K8sResources }
          nodesResources { ...K8sResources }
          containersResources { ...K8sResources }
          resourceWatcher { enable debounceInterval minimumScanInterval watchAllResources resourceTypes }
          containerRepositoriesAllowList
          containerRepositoriesDenyList
          scanCacheEnabled
          scanCacheTtl
          k8sActiveDeadline
          containersActiveDeadline
          jobOverrides { ...K8sJobOverrides }
          scannerJobOverrides { ...K8sJobOverrides }
          nodesJobOverrides { ...K8sJobOverrides }
          containersJobOverrides { ...K8sJobOverrides }
          assetAnnotations
          nodesPriorityClassName
          nodesIntervalTimer
          scannerEnv { name value }
          nodesEnv { name value }
          containersEnv { name value }
        }
      }
    }
  }
}

fragment K8sResources on K8sResourceRequirements {
  cpuRequest
  cpuLimit
  memRequest
  memLimit
}

fragment K8sJobOverrides on K8sJobOverrides {
  ttlSecondsAfterFinished
  annotations
  nodeSelector
  labels
  tolerations { key operator value effect }
}
`

// getKubernetesIntegration reads a Kubernetes integration. It uses queryJSON
// because the configuration holds Map scalars, such as job override annotations.
func (c *ExtendedGqlClient) getKubernetesIntegration(ctx context.Context, mrn string) (*k8sIntegration, error) {
	var data struct {
		ClientIntegration struct {
			Integration k8sIntegration `json:"integration"`
		} `json:"clientIntegration"`
	}
	if err := c.queryJSON(ctx, k8sIntegrationQuery, map[string]any{"mrn": mrn}, &data); err != nil {
		return nil, err
	}

	integration := data.ClientIntegration.Integration
	if integration.Type != string(mondoov1.ClientIntegrationTypeK8s) {
		return nil, fmt.Errorf("integration %s is a %s integration, not a Kubernetes integration", mrn, integration.Type)
	}
	return &integration, nil
}

// model converts the integration into the resource model.
//
// The API returns unset settings as null or as zero values. prior is the state
// the values are read into (nil on import): a zero value from the API stays
// null unless prior holds a value for it, so settings that are left out of the
// configuration don't show a diff.
func (i *k8sIntegration) model(ctx context.Context, prior *integrationKubernetesResourceModel) (integrationKubernetesResourceModel, diag.Diagnostics) {
	var diags diag.Diagnostics
	if prior == nil {
		prior = &integrationKubernetesResourceModel{}
	}
	cfg := i.ConfigurationOptions

	m := integrationKubernetesResourceModel{
		SpaceID:          prior.SpaceID,
		Mrn:              types.StringValue(i.Mrn),
		Name:             types.StringValue(i.Name),
		ScanLocalCluster: types.BoolValue(k8sBoolValue(cfg.ScanLocalCluster)),
	}
	if m.SpaceID.IsNull() || m.SpaceID.IsUnknown() || m.SpaceID.ValueString() == "" {
		m.SpaceID = types.StringValue(Integration{Mrn: i.Mrn}.SpaceID())
	}

	// kubernetes_resources
	{
		p := prior.KubernetesResources
		if p == nil {
			p = &k8sKubernetesResourcesModel{}
		}
		kr := &k8sKubernetesResourcesModel{
			Enable:                types.BoolValue(k8sBoolValue(cfg.ScanWorkloads)),
			Schedule:              k8sStringValue(cfg.Schedule, p.Schedule),
			ActiveDeadlineSeconds: k8sIntValue(cfg.K8sActiveDeadline, p.ActiveDeadlineSeconds),
			JobOverrides:          k8sJobOverridesValue(ctx, cfg.ScannerJobOverrides, p.JobOverrides, &diags),
			ResourceWatcher:       k8sResourceWatcherValue(ctx, cfg.ResourceWatcher, p.ResourceWatcher, &diags),
		}
		if prior.KubernetesResources != nil || kr.Enable.ValueBool() || !k8sAllNull(kr.Schedule, kr.ActiveDeadlineSeconds) || kr.JobOverrides != nil || kr.ResourceWatcher != nil {
			m.KubernetesResources = kr
		}
	}

	// nodes
	{
		p := prior.Nodes
		if p == nil {
			p = &k8sNodesModel{}
		}
		n := &k8sNodesModel{
			Enable:            types.BoolValue(k8sBoolValue(cfg.ScanNodes)),
			Style:             k8sNodeScanStyleValue(cfg.ScanNodesStyle, p.Style),
			Schedule:          k8sStringValue(cfg.NodesSchedule, p.Schedule),
			IntervalTimer:     k8sIntValue(cfg.NodesIntervalTimer, p.IntervalTimer),
			PriorityClassName: k8sStringValue(cfg.NodesPriorityClassName, p.PriorityClassName),
			Resources:         k8sResourcesValue(cfg.NodesResources, p.Resources),
			JobOverrides:      k8sJobOverridesValue(ctx, cfg.NodesJobOverrides, p.JobOverrides, &diags),
			Env:               k8sEnvValue(cfg.NodesEnv, p.Env),
		}
		if prior.Nodes != nil || n.Enable.ValueBool() || !k8sAllNull(n.Style, n.Schedule, n.IntervalTimer, n.PriorityClassName) || n.Resources != nil || n.JobOverrides != nil || n.Env != nil {
			m.Nodes = n
		}
	}

	// containers
	{
		p := prior.Containers
		if p == nil {
			p = &k8sContainersModel{}
		}
		c := &k8sContainersModel{
			Enable:                types.BoolValue(k8sBoolValue(cfg.ScanPublicImages)),
			Schedule:              k8sStringValue(cfg.ContainersSchedule, p.Schedule),
			ActiveDeadlineSeconds: k8sIntValue(cfg.ContainersActiveDeadline, p.ActiveDeadlineSeconds),
			ScanCache:             k8sScanCacheValue(cfg.ScanCacheEnabled, cfg.ScanCacheTTL, p.ScanCache),
			Repositories:          k8sIncludeExcludeValue(ctx, cfg.ContainerRepositoriesAllowList, cfg.ContainerRepositoriesDenyList, p.Repositories, &diags),
			Resources:             k8sResourcesValue(cfg.ContainersResources, p.Resources),
			JobOverrides:          k8sJobOverridesValue(ctx, cfg.ContainersJobOverrides, p.JobOverrides, &diags),
			Env:                   k8sEnvValue(cfg.ContainersEnv, p.Env),
		}
		if prior.Containers != nil || c.Enable.ValueBool() || !k8sAllNull(c.Schedule, c.ActiveDeadlineSeconds) || c.ScanCache != nil || c.Repositories != nil || c.Resources != nil || c.JobOverrides != nil || c.Env != nil {
			m.Containers = c
		}
	}

	// scanner
	{
		p := prior.Scanner
		if p == nil {
			p = &k8sScannerModel{}
		}
		s := &k8sScannerModel{
			Replicas:                        k8sIntValue(cfg.ScannerReplicas, p.Replicas),
			PrivateRegistriesPullSecretRefs: k8sStringListValue(ctx, cfg.PrivateRegistriesPullSecretRefs, p.PrivateRegistriesPullSecretRefs, &diags),
			Resources:                       k8sResourcesValue(cfg.ScannerResources, p.Resources),
			Env:                             k8sEnvValue(cfg.ScannerEnv, p.Env),
		}
		if prior.Scanner != nil || !k8sAllNull(s.Replicas, s.PrivateRegistriesPullSecretRefs) || s.Resources != nil || s.Env != nil {
			m.Scanner = s
		}
	}

	m.Namespaces = k8sIncludeExcludeValue(ctx, cfg.NamespaceAllowList, cfg.NamespaceDenyList, prior.Namespaces, &diags)
	m.JobOverrides = k8sJobOverridesValue(ctx, cfg.JobOverrides, prior.JobOverrides, &diags)
	m.AssetAnnotations = k8sMapValue(ctx, cfg.AssetAnnotations, prior.AssetAnnotations, &diags)

	return m, diags
}

func k8sBoolValue(v *bool) bool {
	return v != nil && *v
}

// k8sKnown reports whether the prior value is set, so that an empty value from
// the API must be kept instead of becoming null.
func k8sKnown(v attr.Value) bool {
	return !v.IsNull() && !v.IsUnknown()
}

func k8sAllNull(values ...attr.Value) bool {
	for _, v := range values {
		if !v.IsNull() {
			return false
		}
	}
	return true
}

func k8sStringValue(v *string, prior types.String) types.String {
	if v != nil && *v != "" {
		return types.StringValue(*v)
	}
	if k8sKnown(prior) && prior.ValueString() == "" {
		return types.StringValue("")
	}
	return types.StringNull()
}

func k8sIntValue(v *int32, prior types.Int32) types.Int32 {
	if v != nil && *v != 0 {
		return types.Int32Value(*v)
	}
	if k8sKnown(prior) && prior.ValueInt32() == 0 {
		return types.Int32Value(0)
	}
	return types.Int32Null()
}

func k8sOptionalBoolValue(v *bool, prior types.Bool) types.Bool {
	if v != nil && *v {
		return types.BoolValue(true)
	}
	if k8sKnown(prior) {
		return types.BoolValue(false)
	}
	return types.BoolNull()
}

func k8sStringListValue(ctx context.Context, v []string, prior types.List, diags *diag.Diagnostics) types.List {
	if len(v) == 0 && !k8sKnown(prior) {
		return types.ListNull(types.StringType)
	}
	if v == nil {
		v = []string{}
	}
	list, d := types.ListValueFrom(ctx, types.StringType, v)
	diags.Append(d...)
	return list
}

func k8sMapValue(ctx context.Context, v map[string]string, prior types.Map, diags *diag.Diagnostics) types.Map {
	if len(v) == 0 && !k8sKnown(prior) {
		return types.MapNull(types.StringType)
	}
	if v == nil {
		v = map[string]string{}
	}
	m, d := types.MapValueFrom(ctx, types.StringType, v)
	diags.Append(d...)
	return m
}

// k8sNodeScanStyleValue converts the API's node scan style. The API reports an
// unset style as CRONJOB, so that value stays null unless prior holds a style.
func k8sNodeScanStyleValue(style string, prior types.String) types.String {
	s := strings.ToLower(style)
	if _, ok := k8sNodeScanStyles[s]; !ok {
		return types.StringNull()
	}
	if s == "cronjob" && !k8sKnown(prior) {
		return types.StringNull()
	}
	return types.StringValue(s)
}

func k8sResourceQuantitiesValue(cpu, memory *string, prior *k8sResourceQuantitiesModel) *k8sResourceQuantitiesModel {
	p := prior
	if p == nil {
		p = &k8sResourceQuantitiesModel{}
	}
	q := &k8sResourceQuantitiesModel{
		CPU:    k8sStringValue(cpu, p.CPU),
		Memory: k8sStringValue(memory, p.Memory),
	}
	if prior == nil && k8sAllNull(q.CPU, q.Memory) {
		return nil
	}
	return q
}

func k8sResourcesValue(v *k8sResourceRequirementsAPI, prior *k8sResourcesModel) *k8sResourcesModel {
	if v == nil {
		v = &k8sResourceRequirementsAPI{}
	}
	p := prior
	if p == nil {
		p = &k8sResourcesModel{}
	}
	r := &k8sResourcesModel{
		Requests: k8sResourceQuantitiesValue(v.CPURequest, v.MemRequest, p.Requests),
		Limits:   k8sResourceQuantitiesValue(v.CPULimit, v.MemLimit, p.Limits),
	}
	if prior == nil && r.Requests == nil && r.Limits == nil {
		return nil
	}
	return r
}

func k8sResourceWatcherValue(ctx context.Context, v *k8sResourceWatcherAPI, prior *k8sResourceWatcherModel, diags *diag.Diagnostics) *k8sResourceWatcherModel {
	if v == nil {
		v = &k8sResourceWatcherAPI{}
	}
	p := prior
	if p == nil {
		p = &k8sResourceWatcherModel{}
	}
	rw := &k8sResourceWatcherModel{
		Enable:              k8sOptionalBoolValue(v.Enable, p.Enable),
		WatchAllResources:   k8sOptionalBoolValue(v.WatchAllResources, p.WatchAllResources),
		ResourceTypes:       k8sStringListValue(ctx, v.ResourceTypes, p.ResourceTypes, diags),
		DebounceInterval:    k8sStringValue(v.DebounceInterval, p.DebounceInterval),
		MinimumScanInterval: k8sStringValue(v.MinimumScanInterval, p.MinimumScanInterval),
	}
	if prior == nil && k8sAllNull(rw.Enable, rw.WatchAllResources, rw.ResourceTypes, rw.DebounceInterval, rw.MinimumScanInterval) {
		return nil
	}
	return rw
}

func k8sScanCacheValue(enabled *bool, ttl *string, prior *k8sScanCacheModel) *k8sScanCacheModel {
	p := prior
	if p == nil {
		p = &k8sScanCacheModel{}
	}
	sc := &k8sScanCacheModel{
		Enable: types.BoolValue(k8sBoolValue(enabled)),
		TTL:    k8sStringValue(ttl, p.TTL),
	}
	if prior == nil && !sc.Enable.ValueBool() && sc.TTL.IsNull() {
		return nil
	}
	return sc
}

func k8sIncludeExcludeValue(ctx context.Context, include, exclude []string, prior *k8sIncludeExcludeModel, diags *diag.Diagnostics) *k8sIncludeExcludeModel {
	p := prior
	if p == nil {
		p = &k8sIncludeExcludeModel{}
	}
	ie := &k8sIncludeExcludeModel{
		Include: k8sStringListValue(ctx, include, p.Include, diags),
		Exclude: k8sStringListValue(ctx, exclude, p.Exclude, diags),
	}
	if prior == nil && k8sAllNull(ie.Include, ie.Exclude) {
		return nil
	}
	return ie
}

func k8sJobOverridesValue(ctx context.Context, v *k8sJobOverridesAPI, prior *k8sJobOverridesModel, diags *diag.Diagnostics) *k8sJobOverridesModel {
	if v == nil {
		v = &k8sJobOverridesAPI{}
	}
	p := prior
	if p == nil {
		p = &k8sJobOverridesModel{}
	}
	jo := &k8sJobOverridesModel{
		TTLSecondsAfterFinished: k8sIntValue(v.TTLSecondsAfterFinished, p.TTLSecondsAfterFinished),
		Annotations:             k8sMapValue(ctx, v.Annotations, p.Annotations, diags),
		Labels:                  k8sMapValue(ctx, v.Labels, p.Labels, diags),
		NodeSelector:            k8sMapValue(ctx, v.NodeSelector, p.NodeSelector, diags),
		Tolerations:             k8sTolerationsValue(v.Tolerations, p.Tolerations),
	}
	if prior == nil && k8sAllNull(jo.TTLSecondsAfterFinished, jo.Annotations, jo.Labels, jo.NodeSelector) && jo.Tolerations == nil {
		return nil
	}
	return jo
}

func k8sTolerationsValue(v []k8sTolerationAPI, prior []k8sTolerationModel) []k8sTolerationModel {
	if len(v) == 0 {
		if prior != nil {
			return []k8sTolerationModel{}
		}
		return nil
	}
	tolerations := make([]k8sTolerationModel, 0, len(v))
	for idx, t := range v {
		var p k8sTolerationModel
		if idx < len(prior) {
			p = prior[idx]
		}
		tolerations = append(tolerations, k8sTolerationModel{
			Key:      k8sStringValue(t.Key, p.Key),
			Operator: k8sStringValue(t.Operator, p.Operator),
			Value:    k8sStringValue(t.Value, p.Value),
			Effect:   k8sStringValue(t.Effect, p.Effect),
		})
	}
	return tolerations
}

func k8sEnvValue(v []k8sEnvVarAPI, prior []k8sEnvVarModel) []k8sEnvVarModel {
	if len(v) == 0 {
		if prior != nil {
			return []k8sEnvVarModel{}
		}
		return nil
	}
	env := make([]k8sEnvVarModel, 0, len(v))
	for idx, e := range v {
		var p k8sEnvVarModel
		if idx < len(prior) {
			p = prior[idx]
		}
		env = append(env, k8sEnvVarModel{
			Name:  types.StringValue(e.Name),
			Value: k8sStringValue(e.Value, p.Value),
		})
	}
	return env
}
