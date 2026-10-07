// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/stretchr/testify/require"
)

const testK8sIntegrationMrn = "//integration.api.mondoo.app/spaces/hungry-poet-123456/integrations/2Abd08lk860"

func k8sTestSchemaState(t *testing.T, model *integrationKubernetesResourceModel) tftypes.Value {
	t.Helper()
	ctx := context.Background()

	var resp resource.SchemaResponse
	(&integrationKubernetesResource{}).Schema(ctx, resource.SchemaRequest{}, &resp)
	require.False(t, resp.Diagnostics.HasError(), "%v", resp.Diagnostics)
	require.False(t, resp.Schema.ValidateImplementation(ctx).HasError())

	state := tfsdk.State{
		Schema: resp.Schema,
		Raw:    tftypes.NewValue(resp.Schema.Type().TerraformType(ctx), nil),
	}
	diags := state.Set(ctx, model)
	require.False(t, diags.HasError(), "%v", diags)
	return state.Raw
}

// k8sTestAPIRoundTrip sends the model's configuration through JSON, the way
// the API stores it, and reads it back like the GraphQL query does.
func k8sTestAPIRoundTrip(t *testing.T, model integrationKubernetesResourceModel) *k8sIntegration {
	t.Helper()
	opts, diags := model.configurationInput(context.Background())
	require.False(t, diags.HasError(), "%v", diags)

	raw, err := json.Marshal(opts.K8sConfigurationOptions)
	require.NoError(t, err)

	integration := &k8sIntegration{Mrn: testK8sIntegrationMrn, Name: model.Name.ValueString(), Type: "K8S"}
	require.NoError(t, json.Unmarshal(raw, &integration.ConfigurationOptions))

	// The API reports an unset node scan style as CRONJOB.
	if integration.ConfigurationOptions.ScanNodesStyle == "" {
		integration.ConfigurationOptions.ScanNodesStyle = "CRONJOB"
	}
	return integration
}

func k8sTestStringList(values ...string) types.List {
	return types.ListValueMust(types.StringType, ConvertListValue(values).Elements())
}

func k8sTestStringMap(values map[string]string) types.Map {
	m, _ := types.MapValueFrom(context.Background(), types.StringType, values)
	return m
}

func k8sTestMinimalModel() integrationKubernetesResourceModel {
	return integrationKubernetesResourceModel{
		SpaceID:          types.StringValue("hungry-poet-123456"),
		Mrn:              types.StringValue(testK8sIntegrationMrn),
		Name:             types.StringValue("eks-minimal"),
		ScanLocalCluster: types.BoolValue(true),
		AssetAnnotations: types.MapNull(types.StringType),
	}
}

func k8sTestFullModel() integrationKubernetesResourceModel {
	jobOverrides := func(ttl int32) *k8sJobOverridesModel {
		return &k8sJobOverridesModel{
			TTLSecondsAfterFinished: types.Int32Value(ttl),
			Annotations:             k8sTestStringMap(map[string]string{"karpenter.sh/do-not-disrupt": "true"}),
			Labels:                  types.MapNull(types.StringType),
			NodeSelector:            k8sTestStringMap(map[string]string{"workload-type": "mondoo-scan"}),
			Tolerations: []k8sTolerationModel{{
				Key:      types.StringValue("CriticalAddonsOnly"),
				Operator: types.StringValue("Equal"),
				Value:    types.StringValue("true"),
				Effect:   types.StringValue("NoSchedule"),
			}},
		}
	}
	resources := func(cpu, memory, memoryLimit string) *k8sResourcesModel {
		return &k8sResourcesModel{
			Requests: &k8sResourceQuantitiesModel{CPU: types.StringValue(cpu), Memory: types.StringValue(memory)},
			Limits:   &k8sResourceQuantitiesModel{CPU: types.StringNull(), Memory: types.StringValue(memoryLimit)},
		}
	}

	m := k8sTestMinimalModel()
	m.Name = types.StringValue("eks-team-a")
	m.KubernetesResources = &k8sKubernetesResourcesModel{
		Enable:                types.BoolValue(true),
		Schedule:              types.StringValue("0 */6 * * *"),
		ActiveDeadlineSeconds: types.Int32Value(3600),
		JobOverrides:          jobOverrides(300),
		ResourceWatcher: &k8sResourceWatcherModel{
			Enable:              types.BoolValue(true),
			WatchAllResources:   types.BoolValue(false),
			ResourceTypes:       k8sTestStringList("deployments", "pods"),
			DebounceInterval:    types.StringValue("10s"),
			MinimumScanInterval: types.StringNull(),
		},
	}
	m.Nodes = &k8sNodesModel{
		Enable:            types.BoolValue(true),
		Style:             types.StringValue("daemonset"),
		Schedule:          types.StringNull(),
		IntervalTimer:     types.Int32Value(720),
		PriorityClassName: types.StringValue("mondoo-low-priority"),
		Resources:         resources("100m", "256Mi", "1Gi"),
		JobOverrides:      nil,
		Env:               []k8sEnvVarModel{{Name: types.StringValue("DEBUG"), Value: types.StringValue("1")}},
	}
	m.Containers = &k8sContainersModel{
		Enable:                types.BoolValue(true),
		Schedule:              types.StringValue("0 0 * * *"),
		ActiveDeadlineSeconds: types.Int32Null(),
		ScanCache:             &k8sScanCacheModel{Enable: types.BoolValue(true), TTL: types.StringValue("8760h")},
		Repositories: &k8sIncludeExcludeModel{
			Include: types.ListNull(types.StringType),
			Exclude: k8sTestStringList("docker.io/library"),
		},
		Resources:    resources("200m", "1Gi", "1Gi"),
		JobOverrides: jobOverrides(300),
		Env:          nil,
	}
	m.Scanner = &k8sScannerModel{
		Replicas:                        types.Int32Value(2),
		PrivateRegistriesPullSecretRefs: k8sTestStringList("mondoo-private-registries-secrets"),
		Resources:                       resources("100m", "256Mi", "2Gi"),
		Env:                             []k8sEnvVarModel{{Name: types.StringValue("HTTPS_PROXY"), Value: types.StringNull()}},
	}
	m.Namespaces = &k8sIncludeExcludeModel{
		Include: types.ListNull(types.StringType),
		Exclude: k8sTestStringList("kube-system"),
	}
	m.AssetAnnotations = k8sTestStringMap(map[string]string{"account": "team-a"})
	return m
}

func TestK8sIntegrationRoundTrip(t *testing.T) {
	ctx := context.Background()
	cases := map[string]integrationKubernetesResourceModel{
		"minimal": k8sTestMinimalModel(),
		"full":    k8sTestFullModel(),
		"explicit empty values": func() integrationKubernetesResourceModel {
			m := k8sTestMinimalModel()
			m.ScanLocalCluster = types.BoolValue(false)
			m.Nodes = &k8sNodesModel{
				Enable:            types.BoolValue(false),
				Style:             types.StringValue("cronjob"),
				Schedule:          types.StringNull(),
				IntervalTimer:     types.Int32Null(),
				PriorityClassName: types.StringNull(),
				Env:               []k8sEnvVarModel{},
			}
			m.Namespaces = &k8sIncludeExcludeModel{
				Include: k8sTestStringList(),
				Exclude: types.ListNull(types.StringType),
			}
			m.JobOverrides = &k8sJobOverridesModel{
				TTLSecondsAfterFinished: types.Int32Value(0),
				Annotations:             k8sTestStringMap(map[string]string{}),
				Labels:                  types.MapNull(types.StringType),
				NodeSelector:            types.MapNull(types.StringType),
				Tolerations:             []k8sTolerationModel{},
			}
			m.AssetAnnotations = k8sTestStringMap(map[string]string{})
			return m
		}(),
	}

	for name, model := range cases {
		t.Run(name, func(t *testing.T) {
			want := k8sTestSchemaState(t, &model)

			integration := k8sTestAPIRoundTrip(t, model)
			got, diags := integration.model(ctx, &model)
			require.False(t, diags.HasError(), "%v", diags)

			require.True(t, want.Equal(k8sTestSchemaState(t, &got)), "read after apply must not produce a diff:\nwant %s\n got %s", want, k8sTestSchemaState(t, &got))
		})
	}
}

func TestK8sIntegrationImport(t *testing.T) {
	ctx := context.Background()

	// Import has no prior state, so an optional setting that is explicitly false,
	// zero or empty can't be told apart from an unset one and imports as null.
	full := k8sTestFullModel()
	fullImported := k8sTestFullModel()
	fullImported.KubernetesResources.ResourceWatcher.WatchAllResources = types.BoolNull()

	for name, tc := range map[string]struct {
		config, imported integrationKubernetesResourceModel
	}{
		"minimal": {config: k8sTestMinimalModel(), imported: k8sTestMinimalModel()},
		"full":    {config: full, imported: fullImported},
	} {
		t.Run(name, func(t *testing.T) {
			integration := k8sTestAPIRoundTrip(t, tc.config)
			got, diags := integration.model(ctx, nil)
			require.False(t, diags.HasError(), "%v", diags)
			require.True(t,
				k8sTestSchemaState(t, &tc.imported).Equal(k8sTestSchemaState(t, &got)),
				"import must reproduce the configuration:\nwant %s\n got %s", k8sTestSchemaState(t, &tc.imported), k8sTestSchemaState(t, &got),
			)
		})
	}
}

func TestK8sIntegrationDrift(t *testing.T) {
	ctx := context.Background()
	prior := k8sTestFullModel()

	integration := k8sTestAPIRoundTrip(t, prior)
	cfg := &integration.ConfigurationOptions
	cfg.ScanNodesStyle = "CRONJOB"
	cfg.ScannerJobOverrides.NodeSelector = nil
	cfg.AssetAnnotations["team"] = "platform"
	cfg.NamespaceDenyList = nil

	got, diags := integration.model(ctx, &prior)
	require.False(t, diags.HasError(), "%v", diags)

	require.Equal(t, "cronjob", got.Nodes.Style.ValueString())
	require.False(t, got.KubernetesResources.JobOverrides.NodeSelector.IsNull())
	require.Empty(t, got.KubernetesResources.JobOverrides.NodeSelector.Elements())
	require.Len(t, got.AssetAnnotations.Elements(), 2)
	require.Empty(t, got.Namespaces.Exclude.Elements())
}

func TestK8sIntegrationReadAPIResponse(t *testing.T) {
	ctx := context.Background()

	// Response of the integration query for an integration the operator created.
	payload := `{
		"mrn": "` + testK8sIntegrationMrn + `",
		"name": "mondoo-operator-1a2b3c4d",
		"type": "K8S",
		"configurationOptions": {
			"scanNodes": true,
			"scanNodesStyle": "DAEMONSET",
			"scanWorkloads": true,
			"scanPublicImages": true,
			"scanLocalCluster": true,
			"namespaceAllowList": null,
			"namespaceDenyList": null,
			"privateRegistriesPullSecretRefs": null,
			"schedule": "0 */6 * * *",
			"nodesSchedule": "9 * * * *",
			"containersSchedule": "0 0 * * *",
			"scannerReplicas": null,
			"scannerResources": {"cpuRequest": "100m", "cpuLimit": null, "memRequest": "256Mi", "memLimit": "2Gi"},
			"nodesResources": null,
			"containersResources": null,
			"resourceWatcher": null,
			"containerRepositoriesAllowList": null,
			"containerRepositoriesDenyList": null,
			"scanCacheEnabled": true,
			"scanCacheTtl": "8760h0m0s",
			"k8sActiveDeadline": null,
			"containersActiveDeadline": null,
			"jobOverrides": null,
			"scannerJobOverrides": {
				"ttlSecondsAfterFinished": 300,
				"annotations": {"karpenter.sh/do-not-disrupt": "true"},
				"nodeSelector": {"workload-type": "mondoo-scan"},
				"labels": null,
				"tolerations": [{"key": "CriticalAddonsOnly", "operator": "Equal", "value": "true", "effect": "NoSchedule"}]
			},
			"nodesJobOverrides": null,
			"containersJobOverrides": null,
			"assetAnnotations": null,
			"nodesPriorityClassName": "mondoo-low-priority",
			"nodesIntervalTimer": 720,
			"scannerEnv": null,
			"nodesEnv": null,
			"containersEnv": null
		}
	}`

	var integration k8sIntegration
	require.NoError(t, json.Unmarshal([]byte(payload), &integration))

	got, diags := integration.model(ctx, nil)
	require.False(t, diags.HasError(), "%v", diags)
	k8sTestSchemaState(t, &got)

	require.Equal(t, "hungry-poet-123456", got.SpaceID.ValueString())
	require.Equal(t, "daemonset", got.Nodes.Style.ValueString())
	require.Equal(t, int32(720), got.Nodes.IntervalTimer.ValueInt32())
	require.Equal(t, "9 * * * *", got.Nodes.Schedule.ValueString())
	require.Equal(t, "8760h0m0s", got.Containers.ScanCache.TTL.ValueString())
	require.Equal(t, "2Gi", got.Scanner.Resources.Limits.Memory.ValueString())
	require.True(t, got.Scanner.Resources.Limits.CPU.IsNull())
	require.Nil(t, got.Nodes.Resources)
	require.Nil(t, got.Namespaces)
	require.Nil(t, got.JobOverrides)
	require.Len(t, got.KubernetesResources.JobOverrides.Tolerations, 1)
	require.Equal(t, "mondoo-scan", got.KubernetesResources.JobOverrides.NodeSelector.Elements()["workload-type"].(types.String).ValueString())
	require.True(t, got.AssetAnnotations.IsNull())
}

func TestK8sIntegrationUnsupportedNodeScanStyle(t *testing.T) {
	m := k8sTestMinimalModel()
	m.Nodes = &k8sNodesModel{
		Enable: types.BoolValue(true),
		Style:  types.StringValue("deployment"),
	}

	_, diags := m.configurationInput(context.Background())
	require.True(t, diags.HasError())
	require.Equal(t, "Unsupported node scan style", diags.Errors()[0].Summary())
}
