config {
  disabled_by_default = false
}

plugin "terraform" {
  enabled = true
}

rule "terraform_comment_syntax" {
  enabled = true
}

rule "terraform_deprecated_index" {
  enabled = true
}

rule "terraform_deprecated_interpolation" {
  enabled = true
}

rule "terraform_deprecated_lookup" {
  enabled = true
}

rule "terraform_empty_list_equality" {
  enabled = true
}

rule "terraform_module_pinned_source" {
  enabled = true
}

rule "terraform_module_version" {
  enabled = true
}

rule "terraform_required_providers" {
  enabled = true
}

rule "terraform_typed_variables" {
  enabled = true
}

rule "terraform_unused_declarations" {
  enabled = true
}

rule "terraform_workspace_remote" {
  enabled = true
}

rule "terraform_required_version" {
  enabled = false
}

rule "terraform_standard_module_structure" {
  enabled = false
}