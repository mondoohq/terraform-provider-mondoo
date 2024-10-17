PLUGINS_DIR="$(HOME)/.terraform.d/plugins"
DEV_BIN_PATH="registry.terraform.io/mondoohq/mondoo/99.0.0/$$(go env GOOS)_$$(go env GOARCH)/terraform-provider-mondoo_v99.0.0"

default: testacc

default: build

build:
	go build -v ./...

install: build
	go install -v ./...

# See https://golangci-lint.run/
lint: ## Runs go linter
	golangci-lint run

# Generate docs and copywrite headers
generate: ## Generate or update documentation
	cd tools; go generate .
	go generate ./...

fmt: ## Runs go formatter
	gofmt -s -w -e .

.PHONY: dev/enter
dev/enter: write-terraform-rc ## Updates the terraformrc to point to the DEV_BIN_PATH. Installs the provider to the DEV_BIN_PATH
	mkdir -vp $(PLUGINS_DIR)
	go build -o $(PLUGINS_DIR)/$(DEV_BIN_PATH)

.PHONY: dev/exit
dev/exit: remove-terraform-rc ## Removes development provider package from DEV_BIN_PATH
	@rm -rvf "$(PLUGINS_DIR)/$(DEV_BIN_PATH)"

.PHONY: write-terraform-rc
write-terraform-rc: ## Write to terraformrc file to mirror mondoohq/mondoo to DEV_BIN_PATH
	scripts/mirror-provider.sh

.PHONY: remove-terraform-rc
remove-terraform-rc: ## Remove the terraformrc file
	@rm -vf "$(HOME)/.terraformrc"

help: ## Show this help
	@grep -E '^([a-zA-Z_/-]+):.*## ' $(MAKEFILE_LIST) | awk -F ':.*## ' '{printf "%-20s %s\n", $$1, $$2}'

test: ## Runs go tests
	go test -v -cover -timeout=120s -parallel=4 ./...

hcl/fmt: ## Runs terraform formatter
	terraform fmt -recursive

hcl/lint: ## Runs terraform linter
	tflint --recursive --config $(PWD)/.tflint.hcl

# Run acceptance tests
testacc:
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

license: license/headers/check

license/headers/check:
	copywrite headers --plan

license/headers/apply:
	copywrite headers

.PHONY: build install lint generate fmt test testacc license license/headers/check license/headers/apply
