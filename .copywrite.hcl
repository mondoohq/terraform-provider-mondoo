schema_version = 1

project {
  license          = "BUSL-1.1"
  copyright_holder = "Mondoo, Inc."
  copyright_year   = 2023

  header_ignore = [
    # examples used within documentation (prose)
    "examples/**",

    # GitHub issue template configuration
    ".github/ISSUE_TEMPLATE/*.yml",

    # golangci-lint tooling configuration
    ".golangci.yml",

    # GoReleaser tooling configuration
    ".goreleaser.yml",
  ]
}
