output "complete_space_setup" {
  value     = [for count, space in mondoo_space.my_space : { "space-name" : space.name, "space-id" : space.id, "token" : mondoo_registration_token.token[count].result }]
  sensitive = true
}
