terraform {
  required_providers {
    mondoo = {
      source  = "mondoohq/mondoo"
      version = ">= 0.1.0"
    }
    google = {
      source  = "hashicorp/google"
      version = "~> 6.0"
    }
  }
}

variable "gcp_project" {
  description = "GCP project ID for the audit log export bucket"
  type        = string
}

variable "gcp_region" {
  description = "GCP region for the bucket"
  type        = string
  default     = "us-central1"
}

variable "mondoo_org_id" {
  description = "Mondoo organization ID (not MRN, just the slug)"
  type        = string
}

variable "wif_pool_id" {
  description = "Workload Identity Pool ID (must already exist)"
  type        = string
  default     = "mondoo-local-dev"
}

variable "wif_provider_id" {
  description = "Workload Identity Pool Provider ID (must already exist)"
  type        = string
  default     = "mondoo-local"
}

variable "bucket_name" {
  description = "Name for the GCS bucket. Must be globally unique."
  type        = string
  default     = "mondoo-audit-log-export"
}

provider "google" {
  project = var.gcp_project
  region  = var.gcp_region
}

provider "mondoo" {}
