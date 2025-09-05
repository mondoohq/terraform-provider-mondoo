
variable "dataset_id" {
  description = "The Bigquery DatasetId"
  type        = string
}
variable "service_account" {
  description = "The Bigquery ServiceAccount"
  type        = string
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Setup the Bigquery integration
resource "mondoo_integration_bigquery" "example" {
  name            = "Bigquery Integration"
  dataset_id      = var.dataset_id
  service_account = var.service_account
}
