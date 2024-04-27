default: testacc

default: build

build:
	go build -v ./...

install: build
	go install -v ./...

# See https://golangci-lint.run/
lint:
	golangci-lint run

# Generate docs and copywrite headers
generate:
	cd tools; go generate .
	go generate ./...

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=4 ./...

hcl/fmt:
	terraform fmt -recursive

hcl/lint:
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
