data "mondoo_policy" "policy" {
  space_id = "eu-goofy-lamport-129172"
}

# output "policy" {
#   value = data.mondoo_policy.policy
# }

output "policy" {
  value = [
    for policy in tolist(data.mondoo_policy.policy.policies) : {
      policy_mrn = policy.policy_mrn
    }
  ]
}

# https://github.com/mondoohq/samples/blob/main/graphql-api/policies_querypacks_frameworks/list_available_policies_query_packs.bru