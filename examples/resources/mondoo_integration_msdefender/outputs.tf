output "cert_pem" {
  description = "The self-signed certificate in PEM format"
  value       = tls_self_signed_cert.credential.cert_pem
  sensitive   = true
}

output "private_key_pem" {
  description = "The private key in PEM format"
  value       = join("\n", [tls_self_signed_cert.credential.cert_pem, tls_private_key.credential.private_key_pem])
  sensitive   = true
}

output "available_subscriptions" {
  description = "Azure Subscriptions"
  value       = data.azurerm_subscriptions.available.subscriptions
}

output "cnspec" {
  description = "cnspec cli command"
  value       = "terraform output -raw private_key_pem > key.pem\ncnspec scan azure --tenant-id ${var.tenant_id} --client-id ${azuread_application.mondoo_security.client_id} --certificate-path key.pem"
}

