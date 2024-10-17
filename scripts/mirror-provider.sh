#!/usr/bin/env bash

# create a .terraformrc file only if the file is not present
write_terraform_rc() {
  if [[ -f "$HOME/.terraformrc" ]]; then
    return 0
  fi

  cat > "$HOME/.terraformrc" << EOL
provider_installation {
    filesystem_mirror {
        path    = "$HOME/.terraform.d/plugins"
        include = ["mondoohq/mondoo"]
    }
    direct {
        exclude = ["mondoohq/mondoo"]
    }
}
EOL
}

write_terraform_rc "$@" || exit 99
