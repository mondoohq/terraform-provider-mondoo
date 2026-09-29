
variable "dataset_id" {
  description = "The BigQuery dataset ID"
  type        = string
}
variable "service_account" {
  description = "The BigQuery service account"
  type        = string
}

provider "mondoo" {
  space = "hungry-poet-123456"
}

# Set up the BigQuery integration
resource "mondoo_integration_bigquery" "example" {
  name            = "Bigquery Integration"
  dataset_id      = var.dataset_id
  service_account = var.service_account
}
