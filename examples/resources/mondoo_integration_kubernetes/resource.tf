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
    "config"         = base64decode(mondoo_service_account.operator.credential)
    "integrationmrn" = mondoo_integration_kubernetes.cluster.mrn
  }
}

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
        enable     = true
        autoCreate = false
      }
      remoteManaged = true
    }
  }
}
