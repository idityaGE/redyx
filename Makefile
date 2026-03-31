.PHONY: proto proto-lint proto-breaking proto-descriptor build test clean docker-build docker-up docker-down docker-logs docker-rebuild web help \
        k8s-create k8s-delete k8s-build k8s-load k8s-data k8s-monitoring k8s-app k8s-up k8s-down k8s-logs k8s-status k8s-port k8s-validate

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
	@grep -E '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

web:  ## Start the astro development server 
	cd web && bun dev

# ─────────────────────────────────────────────────────────────
# Kubernetes (kind) Targets
# ─────────────────────────────────────────────────────────────

K8S_CLUSTER := redyx
K8S_NAMESPACE_APP := redyx-app
K8S_NAMESPACE_DATA := redyx-data
K8S_NAMESPACE_MON := redyx-monitoring

k8s-create:  ## Create kind cluster and namespaces
	kind create cluster --name $(K8S_CLUSTER) --config deploy/k8s/kind-config.yaml
	kubectl create namespace $(K8S_NAMESPACE_APP) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create namespace $(K8S_NAMESPACE_DATA) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create namespace $(K8S_NAMESPACE_MON) --dry-run=client -o yaml | kubectl apply -f -

k8s-delete:  ## Delete kind cluster
	kind delete cluster --name $(K8S_CLUSTER)

k8s-build:  ## Build all service Docker images
	@for svc in skeleton auth user community post vote comment search notification media moderation spam; do \
		echo "Building $$svc..."; \
		docker build -t redyx/$$svc-service:dev -f deploy/docker/Dockerfile --build-arg SERVICE=$$svc . ; \
	done

k8s-load:  ## Load all images into kind cluster
	@for svc in skeleton auth user community post vote comment search notification media moderation spam; do \
		echo "Loading $$svc..."; \
		kind load docker-image redyx/$$svc-service:dev --name $(K8S_CLUSTER) ; \
	done

k8s-data:  ## Deploy data stores (PostgreSQL, Redis, ScyllaDB, Kafka, Meilisearch, MinIO)
	@echo "Data store deployment - see Plan 02"

k8s-monitoring:  ## Deploy observability stack (Prometheus, Grafana, Loki, Jaeger)
	@echo "Monitoring deployment - see Plan 03"

k8s-app:  ## Deploy all microservices via Helm
	helm upgrade --install redyx-app deploy/k8s/charts/redyx-services \
		-n $(K8S_NAMESPACE_APP) \
		-f deploy/k8s/charts/redyx-services/values-dev.yaml

k8s-up: k8s-create k8s-build k8s-load k8s-data k8s-monitoring k8s-app  ## Full K8s stack deployment

k8s-down:  ## Uninstall Helm releases and delete cluster
	-helm uninstall redyx-app -n $(K8S_NAMESPACE_APP)
	-helm uninstall redyx-monitoring -n $(K8S_NAMESPACE_MON)
	-helm uninstall redyx-data -n $(K8S_NAMESPACE_DATA)
	kind delete cluster --name $(K8S_CLUSTER)

k8s-logs:  ## Tail logs from all app services
	kubectl logs -n $(K8S_NAMESPACE_APP) -l app.kubernetes.io/instance=redyx-app -f --tail=100

k8s-status:  ## Show cluster status
	@echo "=== Namespaces ===" && kubectl get ns
	@echo "\n=== Pods (app) ===" && kubectl get pods -n $(K8S_NAMESPACE_APP) -o wide
	@echo "\n=== Pods (data) ===" && kubectl get pods -n $(K8S_NAMESPACE_DATA) -o wide
	@echo "\n=== Pods (monitoring) ===" && kubectl get pods -n $(K8S_NAMESPACE_MON) -o wide
	@echo "\n=== HPAs ===" && kubectl get hpa -n $(K8S_NAMESPACE_APP)

k8s-port:  ## Port-forward observability UIs (Grafana:3000, Prometheus:9090, Jaeger:16686)
	@echo "Starting port-forwards in background..."
	@kubectl port-forward -n $(K8S_NAMESPACE_MON) svc/prometheus 9090:9090 &
	@kubectl port-forward -n $(K8S_NAMESPACE_MON) svc/grafana 3000:3000 &
	@kubectl port-forward -n $(K8S_NAMESPACE_MON) svc/jaeger 16686:16686 &
	@echo "Grafana: http://localhost:3000 (admin/admin)"
	@echo "Prometheus: http://localhost:9090"
	@echo "Jaeger: http://localhost:16686"

k8s-validate:  ## Run validation script
	./scripts/k8s-validate.sh

.DEFAULT_GOAL := help
