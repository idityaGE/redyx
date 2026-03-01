.PHONY: proto proto-lint proto-breaking proto-descriptor build test clean help

proto: proto-lint  ## Generate Go code + Envoy descriptor from protos
	buf generate
	@mkdir -p deploy/envoy
	buf build -o deploy/envoy/proto.pb

proto-lint:  ## Lint proto files
	buf lint

proto-breaking:  ## Check for breaking changes vs git main
	buf breaking --against '.git#branch=main'

proto-descriptor:  ## Build Envoy descriptor set only
	@mkdir -p deploy/envoy
	buf build -o deploy/envoy/proto.pb

build:  ## Build all services
	go build ./cmd/...

test:  ## Run all tests
	go test ./...

clean:  ## Remove generated files
	rm -rf gen/
	rm -f deploy/envoy/proto.pb

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.DEFAULT_GOAL := help
