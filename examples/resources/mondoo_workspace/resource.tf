provider "mondoo" {
  space = "hungry-poet-123456"
}

resource "mondoo_workspace" "my_workspace" {
  name = "My New Workspace"

  asset_selections = [
    {
      conditions = [
        {
          operator = "AND"
          key_value_condition = {
            field    = "LABELS"
            operator = "CONTAINS"
            values = [
              {
                key   = "environment"
                value = "production"
              }
            ]
          }
        },
        {
          operator = "AND"
          rating_condition = {
            field    = "RISK"
            operator = "EQUAL"
            values   = ["CRITICAL"]
          }
        },
        {
          operator = "AND"
          string_condition = {
            field    = "PLATFORM"
            operator = "EQUAL"
            values   = ["redhat", "debian"]
          }
        }
      ]
    }
  ]
}

