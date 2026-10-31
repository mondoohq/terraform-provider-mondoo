# Import using integration MRN.
# The API does not return the integration's credential: after the import, set credential_mrn
# to it. With client_id and client_secret instead, the next apply stores them as a new credential.
terraform import mondoo_integration_action1.example "//captain.api.mondoo.app/spaces/hungry-poet-123456/integrations/2Abd08lk860"
