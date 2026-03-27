.PHONY: proto proto-lint proto-breaking proto-descriptor build test clean docker-build docker-up docker-down docker-logs docker-rebuild web help

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

docker-build: proto-descriptor  ## Build all Docker images for compose
	docker compose build

docker-up: proto-descriptor  ## Start full local stack in background
	docker compose up -d

docker-down:  ## Stop stack and remove orphan containers
	docker compose down --remove-orphans

docker-logs:  ## Tail logs from all services
	docker compose logs -f --tail=200

docker-rebuild: proto-descriptor  ## Rebuild all images without cache
	docker compose build --no-cache

test:  ## Run all tests
	go test ./...

clean:  ## Remove generated files
	rm -rf gen/
	rm -f deploy/envoy/proto.pb

help:  ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

web:  ## Start the astro development server 
	cd web && bun dev

.DEFAULT_GOAL := help
