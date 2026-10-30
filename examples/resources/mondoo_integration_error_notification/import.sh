# Import an integration's alert by the integration's MRN. The API does not
# return the alert's target, so the first apply after the import sets it.
terraform import mondoo_integration_error_notification.shodan_alert "//captain.api.mondoo.app/spaces/hungry-poet-123456/integrations/2Abd08lk860"
