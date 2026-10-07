// Copyright Mondoo, Inc. 2024, 2026
// SPDX-License-Identifier: BUSL-1.1

package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func TestAccKubernetesIntegrationResource(t *testing.T) {
	addr := "mondoo_integration_kubernetes.test"
	emptyPlanAfterRefresh := resource.ConfigPlanChecks{
		PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
	}
	updateInPlace := resource.ConfigPlanChecks{
		PreApply:             []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
		PostApplyPostRefresh: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			// Validation failure for an unsupported node scan style
			{
				Config:      testAccKubernetesIntegrationInvalidStyleConfig(accSpace.ID()),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
			// Create and Read testing
			{
				Config:           testAccKubernetesIntegrationMinimalConfig(accSpace.ID(), "one"),
				ConfigPlanChecks: emptyPlanAfterRefresh,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "one"),
					resource.TestCheckResourceAttr(addr, "space_id", accSpace.ID()),
					resource.TestCheckResourceAttr(addr, "scan_local_cluster", "true"),
					resource.TestMatchResourceAttr(addr, "mrn", regexp.MustCompile(`^//integration\.api\.mondoo\.app/spaces/.+/integrations/.+$`)),
					resource.TestCheckNoResourceAttr(addr, "nodes"),
				),
			},
			// Update and Read testing
			{
				Config:           testAccKubernetesIntegrationFullConfig(accSpace.ID(), "two"),
				ConfigPlanChecks: updateInPlace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "two"),
					resource.TestCheckResourceAttr(addr, "kubernetes_resources.enable", "true"),
					resource.TestCheckResourceAttr(addr, "kubernetes_resources.job_overrides.node_selector.workload-type", "mondoo-scan"),
					resource.TestCheckResourceAttr(addr, "kubernetes_resources.job_overrides.tolerations.0.key", "CriticalAddonsOnly"),
					resource.TestCheckResourceAttr(addr, "nodes.style", "daemonset"),
					resource.TestCheckResourceAttr(addr, "nodes.interval_timer", "720"),
					resource.TestCheckResourceAttr(addr, "nodes.resources.limits.memory", "1Gi"),
					resource.TestCheckResourceAttr(addr, "containers.scan_cache.ttl", "8760h"),
					resource.TestCheckResourceAttr(addr, "scanner.private_registries_pull_secret_refs.0", "registry-creds"),
					resource.TestCheckResourceAttr(addr, "namespaces.exclude.0", "kube-system"),
					resource.TestCheckResourceAttr(addr, "asset_annotations.account", "team-a"),
				),
			},
			// Import testing
			{
				ResourceName: addr,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					return s.RootModule().Resources[addr].Primary.Attributes["mrn"], nil
				},
				ImportStateVerifyIdentifierAttribute: "mrn",
				ImportState:                          true,
				ImportStateVerify:                    true,
			},
			// Turning node scanning off and dropping blocks updates in place
			{
				Config:           testAccKubernetesIntegrationNodesOffConfig(accSpace.ID(), "three"),
				ConfigPlanChecks: updateInPlace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "three"),
					resource.TestCheckResourceAttr(addr, "nodes.enable", "false"),
					resource.TestCheckNoResourceAttr(addr, "nodes.style"),
					resource.TestCheckNoResourceAttr(addr, "scanner"),
					resource.TestCheckNoResourceAttr(addr, "asset_annotations"),
				),
			},
			// Space configured at the provider level
			{
				Config:           testAccKubernetesIntegrationWithSpaceInProviderConfig(accSpace.ID(), "four"),
				ConfigPlanChecks: updateInPlace,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(addr, "name", "four"),
					resource.TestCheckResourceAttr(addr, "space_id", accSpace.ID()),
				),
			},
			// Delete testing automatically occurs in TestCase
		},
	})
}

func testAccKubernetesIntegrationInvalidStyleConfig(spaceID string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_kubernetes" "test" {
  space_id = %[1]q
  name     = "invalid"

  nodes = {
    enable = true
    style  = "deployment"
  }
}
`, spaceID)
}

func testAccKubernetesIntegrationMinimalConfig(spaceID, name string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_kubernetes" "test" {
  space_id = %[1]q
  name     = %[2]q
}
`, spaceID, name)
}

func testAccKubernetesIntegrationFullConfig(spaceID, name string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_kubernetes" "test" {
  space_id = %[1]q
  name     = %[2]q

  kubernetes_resources = {
    enable   = true
    schedule = "0 */6 * * *"
    job_overrides = {
      ttl_seconds_after_finished = 300
      annotations = {
        "karpenter.sh/do-not-disrupt" = "true"
      }
      node_selector = {
        "workload-type" = "mondoo-scan"
      }
      tolerations = [{
        key      = "CriticalAddonsOnly"
        operator = "Equal"
        value    = "true"
        effect   = "NoSchedule"
      }]
    }
  }

  nodes = {
    enable              = true
    style               = "daemonset"
    interval_timer      = 720
    priority_class_name = "mondoo-low-priority"
    resources = {
      requests = {
        cpu    = "100m"
        memory = "256Mi"
      }
      limits = {
        memory = "1Gi"
      }
    }
  }

  containers = {
    enable   = true
    schedule = "0 0 * * *"
    scan_cache = {
      enable = true
      ttl    = "8760h"
    }
  }

  scanner = {
    private_registries_pull_secret_refs = ["registry-creds"]
    resources = {
      requests = {
        cpu    = "100m"
        memory = "256Mi"
      }
      limits = {
        memory = "2Gi"
      }
    }
  }

  namespaces = {
    exclude = ["kube-system"]
  }

  asset_annotations = {
    account = "team-a"
  }
}
`, spaceID, name)
}

func testAccKubernetesIntegrationNodesOffConfig(spaceID, name string) string {
	return fmt.Sprintf(`
resource "mondoo_integration_kubernetes" "test" {
  space_id = %[1]q
  name     = %[2]q

  kubernetes_resources = {
    enable = true
  }

  nodes = {
    enable = false
  }

  containers = {
    enable = true
  }
}
`, spaceID, name)
}

func testAccKubernetesIntegrationWithSpaceInProviderConfig(spaceID, name string) string {
	return fmt.Sprintf(`
provider "mondoo" {
  space = %[1]q
}

resource "mondoo_integration_kubernetes" "test" {
  name = %[2]q

  kubernetes_resources = {
    enable = true
  }
}
`, spaceID, name)
}
