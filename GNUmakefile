PLUGINS_DIR="$(HOME)/.terraform.d/plugins"
MONDOO_PLUGIN_DIR="registry.terraform.io/mondoohq/mondoo"
DEV_BIN_PATH="99.0.0/$$(go env GOOS)_$$(go env GOARCH)/terraform-provider-mondoo_v99.0.0"

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
dev/enter: write-terraform-rc cleanup-examples ## Updates the terraformrc to point to the DEV_BIN_PATH. Installs the provider to the DEV_BIN_PATH
	mkdir -vp $(PLUGINS_DIR)
	go build -o $(PLUGINS_DIR)/$(MONDOO_PLUGIN_DIR)/$(DEV_BIN_PATH)

.PHONY: dev/exit
dev/exit: remove-terraform-rc ## Removes development provider package from DEV_BIN_PATH
	@rm -rvf "$(PLUGINS_DIR)/$(MONDOO_PLUGIN_DIR)"

.PHONY: write-terraform-rc
write-terraform-rc: ## Write to terraformrc file to mirror mondoohq/mondoo to DEV_BIN_PATH
	scripts/mirror-provider.sh

.PHONY: remove-terraform-rc
remove-terraform-rc: ## Remove the terraformrc file
	@rm -vf "$(HOME)/.terraformrc"

.PHONY: cleanup-examples
cleanup-examples: ## A quick way to clean up any left over Terraform files inside the examples/ folder
	find . -name ".terraform*" -type f -exec rm -rf {} \;
	find . -name "terraform.tfstate*" -type f -exec rm -rf {} \;
	find . -name ".terraform.lock.hcl" -type f -exec rm -rf {} \;

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
	@echo "** Warning: \n  This requires an _organization_ level service account. \n Please set MONDOO_CONFIG_BASE64 env var to your local dev base64 encoded json service account when running tests locally**\n\n"
	TF_ACC=1 go test ./... -v $(TESTARGS) -timeout 120m

license: license/headers/check

license/headers/check:
	copywrite headers --plan

license/headers/apply:
	copywrite headers

.PHONY: build install lint generate fmt test testacc license license/headers/check license/headers/apply
