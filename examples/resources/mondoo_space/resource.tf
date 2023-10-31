terraform {
  required_providers {
    mondoo = {
      source = "mondoo/mondoo"
    }
  }
}

provider "mondoo" {
  region = "us"
}

resource "mondoo_space" "my_space" {
  name   = "My Space New"
  // id = "your-space-id" # optional otherwise it will be auto-generated
  org_id = "youthful-meitner-435985"
}

resource "mondoo_service_account" "service_account" {
  name        = "Service Account Terraform New"
  description = "Service Account for Terraform"
  roles = [
    "//iam.api.mondoo.app/roles/viewer", // TODO use "roles/viewer"
  ]
  space_id = mondoo_space.my_space.id

  depends_on = [
    mondoo_space.my_space
  ]
}

resource "mondoo_integration_oci_tenant" "tenant_abc" {
  space_id    = mondoo_space.my_space.id
  name        = "tenant ABC"

  tenancy   = "ocid1.tenancy.oc1..aaaaaaaabnjfuyr73mmvv6ep7heu57576mtqbhju5ni275c6rrfqiu6q6joq"
  region = "us-ashburn-1"
  user   = "ocid1.user.oc1..aaaaaaaa2od3jeszy2fwgtvsp77s3y2hqaott37qg62ctm56ujf2crdgi5wa"

  credentials = {
    fingerprint = "72:f4:f6:50:16:43:32:1f:40:1e:39:59:eb:77:7b:38"
    private_key = <<EOT
-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQCf2kWtE6JkkP6E
cnQx/1oa4GqFs23nJFBQhgn9AThqAyUC1ilLQV9ZKjQj5/6+ljq/i4H/zU5lt2yB
6IWkpbShnwcRXsSYpHWvU+LW+AVYg2o2NNqrNbzo6jZ8SAIFsyHLaWCvQNl/jT82
nyeysjBHqxF0KjaknhMnB6HW8wWyVtqdJJpxoU0bOeGnLVptDl/Fd3cbEVn6uXNy
9YD+9/moW7q1p4x6LuxpxW3M0vbC+v8ynWnXxrHF2Ztua93Q1yPNrlzr6IM0F4Jw
HPwbR9zTAmk9bhwZhzF5gjoQzbPcgGZAr26kUwQyQziOLEVKCH4JDkD5REWt3n0R
VkSu4eNBAgMBAAECggEAPl/Y5UaujKhJMBxK+8Fkgu1Dk4UmotU4I5uR5S/nN/Kf
LAKuHUfWcT/3FXWuoOaU6O4CXXLanvUaOXymPU9Ncu1PQDdm5hryMuY8Mt6Xk2i/
9oPcOYdf/dB6Tzd31oqF6s3lJK1c0Rkn6+PhhShjj2uQhW/9X3GxDn87ZtjwRZxh
joq5KbEutpHHJ4tNp9wJmbd0ldVBqx91NHOC1mFq3bfyVLC8kxE9d7BOrwT4TCRP
I92b8uHZM0oVmYst+3oZwC89fuK/Z+tz3WyFXh3gePZARUaK5cC+StQvLI6+KxFA
wm/FaqYXIHrPLE1WHYshgoseIstd8A5kRl3X1nqa0wKBgQDOEtlIoZ9E8MKAxrXD
10GZNpo8ztZepiRE5+UnN4qKFA4cGdACfr8Imdu94zZSUPkcvr3emfzDpPOhnucU
tuoHPw4HEmwXX1vu3N3z9rJUYr2E7Ni+BWmV3m20Q6o0XtZ9tlQSk0ivxLYQjide
2vELERb5AvJBeV0tpHDDFggaXwKBgQDGlLFNVWSMdeHmjqUI+K3fcTd93hLaiAgg
g5lDz75uLnNen6fGaLdgV3XwaJd9GNILnXckxDCwk3VkF/BOSGPqucrzBvKKSXrD
lT6X6n1AQKu1wSa4zt6vTmSdGVOK2E/WnUMKPAV96WuLKl1SKS8dLgH5zdoUjGVv
32X44comXwKBgHJS1qSKtZdDkkRq+Q/q7YOYXTz66sabmWd41xJIp90ufx1r3JBl
zIlzAgt4b/x+25Ts5N0HxMitTFQPmddOGstmWdvmhnz49EGx2pir9gcGuGl0FFJn
Ikp4mZf2KgjfzFL1wfKEL0ED+pV4p7Lh9/PRyVLgJZHZSK43mi9Am8I1AoGAUlqa
CTNPxrygmcgwgz72hMLkO4vcj8p4bFuHNUszc2hKKkTWBH+rBQZgf/owUQ35Fh4Q
qiu+8YvF1GPlIeH9pfu1QgJwlY8RnYkIc2Io3Xu0emUHFP+d9F/zc/9r2RoKSjvf
8J+hu20RT56bIxa3VkedRCbtuTXcX3/rP0MMXcsCgYEAgaql3BG5em7CEcz8gETD
cvtS4HWzLxcUEuQeqQBQBJqekXj1aqgYCwQ0RpBUaPkhEPsquiKkrYv321cQdAYH
qpbiCwjFYHmjWFygtYPhRH4T5TEzu4DXhjr4nn99sF0QFKcYkcTSIm7aZppYG4OS
1fnF+XoTcyFIGcSX/I1ND/4=
-----END PRIVATE KEY-----
EOT
  }
}

#resource "mondoo_integration_gcp_organization" "account_abc" {
#  space_id        = mondoo_space.my_space.space_id
#  name            = "account ABC"
#  organization_id = "ABC-project-id"
#  credentials {
#    client_id      = "123456789012345678900"
#    client_email   = "email@abc-project-name.iam.gserviceaccount.com"
#    private_key_id = "1234abcd1234abcd1234abcd1234abcd1234abcd"
#    private_key    = "-----BEGIN PRIVATE KEY-----\n ... -----END PRIVATE KEY-----\n"
#  }
#}
#
#resource "mondoo_integration_gcp_project" "account_abc" {
#  space_id   = mondoo_space.my_space.space_id
#  name       = "account ABC"
#  project_id = "ABC-project-id"
#  credentials {
#    client_id      = "123456789012345678900"
#    client_email   = "email@abc-project-name.iam.gserviceaccount.com"
#    private_key_id = "1234abcd1234abcd1234abcd1234abcd1234abcd"
#    private_key    = "-----BEGIN PRIVATE KEY-----\n ... -----END PRIVATE KEY-----\n"
#  }
#}



#resource "mondoo_integration_azure_subscription" "account_abc" {
#  space_id  = mondoo_space.my_space.space_id
#  name      = "account ABC"
#  tenant_id = "abbc1234-abc1-123a-1234-abcd1234abcd"
#  credentials {
#    client_id     = "1234abcd-abcd-1234-ab12-abcd1234abcd"
#    client_secret = "ABCD1234abcd1234abdc1234ABCD1234abcdefxxx="
#  }
#}