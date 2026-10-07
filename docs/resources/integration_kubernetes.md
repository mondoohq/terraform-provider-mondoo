---
page_title: "mondoo_integration_kubernetes Resource - terraform-provider-mondoo"
subcategory: ""
description: |-
  Manages a Kubernetes integration and the scan configuration of the mondoo-operator that reports to it.
  ~> This resource only works together with a remote-managed operator. The integration holds the scan configuration, and the mondoo-operator (v13.4.0 or later) applies it only when its credentials Secret holds this resource's mrn under the integrationmrn key and its MondooAuditConfig sets spec.remoteManaged: true, spec.consoleIntegration.enable: true and spec.consoleIntegration.autoCreate: false. Without that setup, the operator scans with its own spec and the integration shows a configuration that doesn't run in the cluster.
---

# mondoo_integration_kubernetes (Resource)

Manages a Kubernetes integration and the scan configuration of the mondoo-operator that reports to it.

~> **This resource only works together with a remote-managed operator.** The integration holds the scan configuration, and the mondoo-operator (v13.4.0 or later) applies it only when its credentials Secret holds this resource's `mrn` under the `integrationmrn` key and its MondooAuditConfig sets `spec.remoteManaged: true`, `spec.consoleIntegration.enable: true` and `spec.consoleIntegration.autoCreate: false`. Without that setup, the operator scans with its own spec and the integration shows a configuration that doesn't run in the cluster.

## Operator setup

This resource stores the scan configuration in Mondoo, and the mondoo-operator applies it. That only works when the operator is set up for it, so always change both together:

| Where | Setting | Why |
|---|---|---|
| The Secret referenced by `spec.mondooCredsSecretRef` | `integrationmrn` set to this resource's `mrn`, next to the service account credentials in `config` | The operator checks in with this integration and fetches its configuration. |
| MondooAuditConfig | `spec.remoteManaged: true` | The operator applies the integration's configuration instead of the MondooAuditConfig's own spec. |
| MondooAuditConfig | `spec.consoleIntegration.enable: true` | Turns on check-ins, status reporting and fetching the configuration. It's required because `autoCreate` is off. |
| MondooAuditConfig | `spec.consoleIntegration.autoCreate: false` | Keeps the operator from creating an integration of its own and writing that integration's MRN into the Secret. |

The example below sets up all of them. The MondooAuditConfig needs no scan settings of its own: they all come from this resource.

What happens when part of the setup is missing:

- **No `remoteManaged`:** the operator scans with the MondooAuditConfig's own spec and never applies the integration's configuration, so the integration shows settings that don't run in the cluster.
- **No `integrationmrn` in the Secret:** the operator can't check in. With `autoCreate: false` it reports the missing key in its `IntegrationDegraded` condition. With `autoCreate` left on, it creates an integration of its own and rewrites the Secret; if Terraform manages the Secret, the next apply reverts that, and the operator silently stops checking in.
- **No `consoleIntegration.enable`:** the operator never checks in, so it never fetches the configuration and the integration never becomes active.

Changes reach the cluster on the operator's next check-in, about every 10 minutes. When you switch an existing MondooAuditConfig to `remoteManaged`, the operator applies its now minimal local spec until that first check-in: it removes the scan workloads and recreates them once the configuration arrives.

To check that the operator applies the integration's configuration, look at the MondooAuditConfig's status. It holds the hash of the fetched configuration, and `IntegrationDegraded` is `False`:

```shell
kubectl -n mondoo-operator get mondooauditconfig mondoo-client \
  -o jsonpath='{.status.remoteConfigHash}{"\n"}{.status.conditions[?(@.type=="IntegrationDegraded")].status}{"\n"}'
```

## Example Usage

```terraform
provider "mondoo" {
  space = "hungry-poet-123456"
}

# The integration holds the scan configuration. The operator pulls it on every
# check-in because the MondooAuditConfig below sets remoteManaged.
resource "mondoo_integration_kubernetes" "cluster" {
  name = "eks-production"

  kubernetes_resources = {
    enable   = true
    schedule = "0 */6 * * *"
  }

  nodes = {
    enable         = true
    style          = "daemonset"
    interval_timer = 720
  }

  containers = {
    enable   = true
    schedule = "0 0 * * *"
    scan_cache = {
      enable = true
      ttl    = "24h"
    }
  }

  namespaces = {
    exclude = ["kube-system"]
  }

  asset_annotations = {
    cluster = "eks-production"
  }
}

# Credentials the operator scans and checks in with.
resource "mondoo_service_account" "operator" {
  name  = "eks-production-operator"
  roles = ["//iam.api.mondoo.app/roles/agent"]
}

resource "kubernetes_secret" "mondoo_client" {
  metadata {
    name      = "mondoo-client"
    namespace = "mondoo-operator"
  }

  data = {
    "config" = base64decode(mondoo_service_account.operator.credential)
    # Required: the integration the operator checks in with and pulls its configuration from.
    "integrationmrn" = mondoo_integration_kubernetes.cluster.mrn
  }
}

# The operator only applies the integration's configuration with all three
# consoleIntegration and remoteManaged settings below.
resource "kubernetes_manifest" "mondoo_audit_config" {
  manifest = {
    apiVersion = "k8s.mondoo.com/v1alpha2"
    kind       = "MondooAuditConfig"
    metadata = {
      name      = "mondoo-client"
      namespace = "mondoo-operator"
    }
    spec = {
      mondooCredsSecretRef = {
        name = kubernetes_secret.mondoo_client.metadata[0].name
      }
      consoleIntegration = {
        # Required: check in with the integration from the Secret.
        enable = true
        # Required: Terraform owns the integration, the operator must not create its own.
        autoCreate = false
      }
      # Required: apply the integration's scan configuration instead of this spec.
      remoteManaged = true
    }
  }
}
```

## Limitations

- This resource manages the integration's whole scan configuration. Settings it doesn't support (external clusters, workload identity for container registries, and routing assets to another space) are cleared when Terraform creates or updates the integration.
- On import, an optional setting that is set to `false`, `0` or an empty value can't be told apart from an unset one and is imported as unset. If the configuration sets it explicitly, the next apply updates the integration without changing its behavior.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `name` (String) Name of the integration.

### Optional

- `asset_annotations` (Map of String) Annotations added to every asset the operator scans.
- `containers` (Attributes) Scanning of the container images that run in the cluster. Disabled if unset. (see [below for nested schema](#nestedatt--containers))
- `job_overrides` (Attributes) Job and pod settings for all scan types. The per-type `job_overrides` take precedence. (see [below for nested schema](#nestedatt--job_overrides))
- `kubernetes_resources` (Attributes) Scanning of the cluster's Kubernetes resources, such as Deployments, Pods and Ingresses. Disabled if unset. (see [below for nested schema](#nestedatt--kubernetes_resources))
- `namespaces` (Attributes) Namespaces to scan or skip. (see [below for nested schema](#nestedatt--namespaces))
- `nodes` (Attributes) Scanning of the cluster's nodes. Disabled if unset. (see [below for nested schema](#nestedatt--nodes))
- `scan_local_cluster` (Boolean) Whether the operator scans the cluster it runs in. When `false`, the operator scans neither the cluster's Kubernetes resources nor its nodes. Defaults to `true`.
- `scanner` (Attributes) Settings of the scanner that runs Kubernetes resource scans. (see [below for nested schema](#nestedatt--scanner))
- `space_id` (String) Mondoo space identifier. If there is no space ID, the provider space is used.

### Read-Only

- `mrn` (String) Integration identifier. Store it in the `integrationmrn` key of the operator's credentials Secret.

<a id="nestedatt--containers"></a>
### Nested Schema for `containers`

Required:

- `enable` (Boolean) Whether to scan the container images that run in the cluster.

Optional:

- `active_deadline_seconds` (Number) Seconds after which a running container image scan is stopped.
- `env` (Attributes List) Environment variables for container image scans. (see [below for nested schema](#nestedatt--containers--env))
- `job_overrides` (Attributes) Job and pod settings for container image scans. They take precedence over the top-level `job_overrides`. (see [below for nested schema](#nestedatt--containers--job_overrides))
- `repositories` (Attributes) Container image repositories to scan or skip. (see [below for nested schema](#nestedatt--containers--repositories))
- `resources` (Attributes) Compute resources for container image scans. (see [below for nested schema](#nestedatt--containers--resources))
- `scan_cache` (Attributes) Caching of image scan results, so that an image is scanned again only after the cache entry expires. (see [below for nested schema](#nestedatt--containers--scan_cache))
- `schedule` (String) Cron schedule for container image scans, such as `0 */6 * * *`. If unset, the operator picks a minute of every hour based on the cluster.

<a id="nestedatt--containers--env"></a>
### Nested Schema for `containers.env`

Required:

- `name` (String) Name of the variable.

Optional:

- `value` (String) Value of the variable.


<a id="nestedatt--containers--job_overrides"></a>
### Nested Schema for `containers.job_overrides`

Optional:

- `annotations` (Map of String) Annotations added to the scan Jobs and their pods, such as `karpenter.sh/do-not-disrupt`.
- `labels` (Map of String) Labels added to the scan Jobs and their pods.
- `node_selector` (Map of String) Node selector for the scan pods.
- `tolerations` (Attributes List) Tolerations for the scan pods. (see [below for nested schema](#nestedatt--containers--job_overrides--tolerations))
- `ttl_seconds_after_finished` (Number) Seconds after which finished scan Jobs are deleted.

<a id="nestedatt--containers--job_overrides--tolerations"></a>
### Nested Schema for `containers.job_overrides.tolerations`

Optional:

- `effect` (String) `NoSchedule`, `PreferNoSchedule` or `NoExecute`. Leave it unset to match every effect.
- `key` (String) Taint key the toleration applies to. Leave it unset with operator `Exists` to tolerate every taint.
- `operator` (String) `Equal` or `Exists`. Kubernetes defaults to `Equal`.
- `value` (String) Taint value the toleration matches, with operator `Equal`.



<a id="nestedatt--containers--repositories"></a>
### Nested Schema for `containers.repositories`

Optional:

- `exclude` (List of String) Repositories to skip.
- `include` (List of String) Repositories to scan. If set, images from other repositories are skipped.


<a id="nestedatt--containers--resources"></a>
### Nested Schema for `containers.resources`

Optional:

- `limits` (Attributes) Resource limits, as Kubernetes quantities. (see [below for nested schema](#nestedatt--containers--resources--limits))
- `requests` (Attributes) Resource requests, as Kubernetes quantities. (see [below for nested schema](#nestedatt--containers--resources--requests))

<a id="nestedatt--containers--resources--limits"></a>
### Nested Schema for `containers.resources.limits`

Optional:

- `cpu` (String) CPU, such as `100m`.
- `memory` (String) Memory, such as `256Mi`.


<a id="nestedatt--containers--resources--requests"></a>
### Nested Schema for `containers.resources.requests`

Optional:

- `cpu` (String) CPU, such as `100m`.
- `memory` (String) Memory, such as `256Mi`.



<a id="nestedatt--containers--scan_cache"></a>
### Nested Schema for `containers.scan_cache`

Required:

- `enable` (Boolean) Whether to cache image scan results.

Optional:

- `ttl` (String) How long a cached result stays valid, such as `24h`.



<a id="nestedatt--job_overrides"></a>
### Nested Schema for `job_overrides`

Optional:

- `annotations` (Map of String) Annotations added to the scan Jobs and their pods, such as `karpenter.sh/do-not-disrupt`.
- `labels` (Map of String) Labels added to the scan Jobs and their pods.
- `node_selector` (Map of String) Node selector for the scan pods.
- `tolerations` (Attributes List) Tolerations for the scan pods. (see [below for nested schema](#nestedatt--job_overrides--tolerations))
- `ttl_seconds_after_finished` (Number) Seconds after which finished scan Jobs are deleted.

<a id="nestedatt--job_overrides--tolerations"></a>
### Nested Schema for `job_overrides.tolerations`

Optional:

- `effect` (String) `NoSchedule`, `PreferNoSchedule` or `NoExecute`. Leave it unset to match every effect.
- `key` (String) Taint key the toleration applies to. Leave it unset with operator `Exists` to tolerate every taint.
- `operator` (String) `Equal` or `Exists`. Kubernetes defaults to `Equal`.
- `value` (String) Taint value the toleration matches, with operator `Equal`.



<a id="nestedatt--kubernetes_resources"></a>
### Nested Schema for `kubernetes_resources`

Required:

- `enable` (Boolean) Whether to scan the cluster's Kubernetes resources.

Optional:

- `active_deadline_seconds` (Number) Seconds after which a running Kubernetes resource scan is stopped.
- `job_overrides` (Attributes) Job settings for Kubernetes resource scans. They take precedence over the top-level `job_overrides`. (see [below for nested schema](#nestedatt--kubernetes_resources--job_overrides))
- `resource_watcher` (Attributes) Scans of Kubernetes resources as soon as they change, in addition to the scheduled scans. (see [below for nested schema](#nestedatt--kubernetes_resources--resource_watcher))
- `schedule` (String) Cron schedule for Kubernetes resource scans, such as `0 */6 * * *`. If unset, the operator picks a minute of every hour based on the cluster.

<a id="nestedatt--kubernetes_resources--job_overrides"></a>
### Nested Schema for `kubernetes_resources.job_overrides`

Optional:

- `annotations` (Map of String) Annotations added to the scan Jobs and their pods, such as `karpenter.sh/do-not-disrupt`.
- `labels` (Map of String) Labels added to the scan Jobs and their pods.
- `node_selector` (Map of String) Node selector for the scan pods.
- `tolerations` (Attributes List) Tolerations for the scan pods. (see [below for nested schema](#nestedatt--kubernetes_resources--job_overrides--tolerations))
- `ttl_seconds_after_finished` (Number) Seconds after which finished scan Jobs are deleted.

<a id="nestedatt--kubernetes_resources--job_overrides--tolerations"></a>
### Nested Schema for `kubernetes_resources.job_overrides.tolerations`

Optional:

- `effect` (String) `NoSchedule`, `PreferNoSchedule` or `NoExecute`. Leave it unset to match every effect.
- `key` (String) Taint key the toleration applies to. Leave it unset with operator `Exists` to tolerate every taint.
- `operator` (String) `Equal` or `Exists`. Kubernetes defaults to `Equal`.
- `value` (String) Taint value the toleration matches, with operator `Equal`.



<a id="nestedatt--kubernetes_resources--resource_watcher"></a>
### Nested Schema for `kubernetes_resources.resource_watcher`

Optional:

- `debounce_interval` (String) How long to collect changes before scanning them, such as `10s`. The operator defaults to 10 seconds.
- `enable` (Boolean) Whether to watch Kubernetes resources for changes.
- `minimum_scan_interval` (String) Minimum time between two scans, such as `2m`. The operator defaults to 2 minutes.
- `resource_types` (List of String) Resource types to watch, such as `deployments` or `pods`. If unset, the types follow `watch_all_resources`.
- `watch_all_resources` (Boolean) Whether to watch all resource types, including short-lived ones such as Pods, Jobs and CronJobs. By default, only Deployments, DaemonSets, StatefulSets and ReplicaSets are watched.



<a id="nestedatt--namespaces"></a>
### Nested Schema for `namespaces`

Optional:

- `exclude` (List of String) Namespaces to skip.
- `include` (List of String) Namespaces to scan. If set, all other namespaces are skipped.


<a id="nestedatt--nodes"></a>
### Nested Schema for `nodes`

Required:

- `enable` (Boolean) Whether to scan the cluster's nodes.

Optional:

- `env` (Attributes List) Environment variables for node scans. (see [below for nested schema](#nestedatt--nodes--env))
- `interval_timer` (Number) Minutes between two node scans with the `daemonset` style. The operator defaults to 60.
- `job_overrides` (Attributes) Job and pod settings for node scans. They take precedence over the top-level `job_overrides`. (see [below for nested schema](#nestedatt--nodes--job_overrides))
- `priority_class_name` (String) Name of the PriorityClass of the node scan pods. The PriorityClass must exist in the cluster.
- `resources` (Attributes) Compute resources for node scans. (see [below for nested schema](#nestedatt--nodes--resources))
- `schedule` (String) Cron schedule for node scans with the `cronjob` style, such as `0 */6 * * *`. If unset, the operator picks a minute of every hour based on the cluster.
- `style` (String) How the operator runs node scans: `cronjob` (the default, one Job per node on `schedule`) or `daemonset` (one pod on every node, scanning every `interval_timer` minutes).

<a id="nestedatt--nodes--env"></a>
### Nested Schema for `nodes.env`

Required:

- `name` (String) Name of the variable.

Optional:

- `value` (String) Value of the variable.


<a id="nestedatt--nodes--job_overrides"></a>
### Nested Schema for `nodes.job_overrides`

Optional:

- `annotations` (Map of String) Annotations added to the scan Jobs and their pods, such as `karpenter.sh/do-not-disrupt`.
- `labels` (Map of String) Labels added to the scan Jobs and their pods.
- `node_selector` (Map of String) Node selector for the scan pods.
- `tolerations` (Attributes List) Tolerations for the scan pods. (see [below for nested schema](#nestedatt--nodes--job_overrides--tolerations))
- `ttl_seconds_after_finished` (Number) Seconds after which finished scan Jobs are deleted.

<a id="nestedatt--nodes--job_overrides--tolerations"></a>
### Nested Schema for `nodes.job_overrides.tolerations`

Optional:

- `effect` (String) `NoSchedule`, `PreferNoSchedule` or `NoExecute`. Leave it unset to match every effect.
- `key` (String) Taint key the toleration applies to. Leave it unset with operator `Exists` to tolerate every taint.
- `operator` (String) `Equal` or `Exists`. Kubernetes defaults to `Equal`.
- `value` (String) Taint value the toleration matches, with operator `Equal`.



<a id="nestedatt--nodes--resources"></a>
### Nested Schema for `nodes.resources`

Optional:

- `limits` (Attributes) Resource limits, as Kubernetes quantities. (see [below for nested schema](#nestedatt--nodes--resources--limits))
- `requests` (Attributes) Resource requests, as Kubernetes quantities. (see [below for nested schema](#nestedatt--nodes--resources--requests))

<a id="nestedatt--nodes--resources--limits"></a>
### Nested Schema for `nodes.resources.limits`

Optional:

- `cpu` (String) CPU, such as `100m`.
- `memory` (String) Memory, such as `256Mi`.


<a id="nestedatt--nodes--resources--requests"></a>
### Nested Schema for `nodes.resources.requests`

Optional:

- `cpu` (String) CPU, such as `100m`.
- `memory` (String) Memory, such as `256Mi`.




<a id="nestedatt--scanner"></a>
### Nested Schema for `scanner`

Optional:

- `env` (Attributes List) Environment variables for the scanner. (see [below for nested schema](#nestedatt--scanner--env))
- `private_registries_pull_secret_refs` (List of String) Names of Secrets, in the operator's namespace, that hold credentials for private container registries.
- `replicas` (Number) Number of scanner replicas. The operator defaults to 1.
- `resources` (Attributes) Compute resources for the scanner. (see [below for nested schema](#nestedatt--scanner--resources))

<a id="nestedatt--scanner--env"></a>
### Nested Schema for `scanner.env`

Required:

- `name` (String) Name of the variable.

Optional:

- `value` (String) Value of the variable.


<a id="nestedatt--scanner--resources"></a>
### Nested Schema for `scanner.resources`

Optional:

- `limits` (Attributes) Resource limits, as Kubernetes quantities. (see [below for nested schema](#nestedatt--scanner--resources--limits))
- `requests` (Attributes) Resource requests, as Kubernetes quantities. (see [below for nested schema](#nestedatt--scanner--resources--requests))

<a id="nestedatt--scanner--resources--limits"></a>
### Nested Schema for `scanner.resources.limits`

Optional:

- `cpu` (String) CPU, such as `100m`.
- `memory` (String) Memory, such as `256Mi`.


<a id="nestedatt--scanner--resources--requests"></a>
### Nested Schema for `scanner.resources.requests`

Optional:

- `cpu` (String) CPU, such as `100m`.
- `memory` (String) Memory, such as `256Mi`.

## Import

Import is supported using the following syntax:

The [`terraform import` command](https://developer.hashicorp.com/terraform/cli/commands/import) can be used, for example:

```shell
# Import using integration MRN.
terraform import mondoo_integration_kubernetes.cluster "//integration.api.mondoo.app/spaces/hungry-poet-123456/integrations/2Abd08lk860"
```
