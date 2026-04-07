.PHONY: proto proto-lint proto-breaking proto-descriptor build test clean docker-build docker-up docker-down docker-logs docker-rebuild web help \
        k8s-create k8s-delete k8s-build k8s-load k8s-pull k8s-ingress k8s-ingress-down k8s-storage k8s-data k8s-data-down k8s-monitoring k8s-monitoring-down k8s-app k8s-up k8s-down k8s-logs k8s-status k8s-urls k8s-validate k8s-data-reset

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

DATA_DIR ?= $(HOME)/.redyx-data

K8S_CLUSTER := redyx
K8S_NAMESPACE_APP := redyx-app
K8S_NAMESPACE_DATA := redyx-data
K8S_NAMESPACE_MON := redyx-monitoring
SERVICES := skeleton auth user community post vote comment search notification media moderation spam

k8s-create:  ## Create kind cluster and namespaces
	@mkdir -p $(DATA_DIR)/{postgresql,redis,scylladb,kafka,meilisearch,minio,prometheus,loki,grafana}
	@sed 's|__DATA_DIR__|$(DATA_DIR)|g' deploy/k8s/kind-config.yaml > /tmp/redyx-kind-config.yaml
	kind create cluster --name $(K8S_CLUSTER) --config /tmp/redyx-kind-config.yaml
	kubectl create namespace $(K8S_NAMESPACE_APP) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create namespace $(K8S_NAMESPACE_DATA) --dry-run=client -o yaml | kubectl apply -f -
	kubectl create namespace $(K8S_NAMESPACE_MON) --dry-run=client -o yaml | kubectl apply -f -

k8s-storage:  ## Apply local StorageClass and PersistentVolumes (backed by host ~/.redyx-data)
	kubectl apply -f deploy/k8s/storage/local-storage.yaml

k8s-data-reset:  ## Wipe all persisted data from host (irreversible)
	rm -rf $(DATA_DIR)

k8s-delete:  ## Delete kind cluster
	kind delete cluster --name $(K8S_CLUSTER)

k8s-ingress:  ## Deploy NGINX Ingress Controller
	@echo "Adding NGINX Ingress Helm repo..."
	helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
	helm repo update
	@echo "Creating ingress-nginx namespace..."
	kubectl create namespace ingress-nginx --dry-run=client -o yaml | kubectl apply -f -
	@echo "Deploying NGINX Ingress Controller..."
	helm upgrade --install ingress-nginx ingress-nginx/ingress-nginx \
		-n ingress-nginx \
		-f deploy/k8s/ingress/nginx-values.yaml \
		--wait --timeout 5m
	@echo "Waiting for Ingress Controller to be ready..."
	kubectl wait --namespace ingress-nginx \
		--for=condition=ready pod \
		--selector=app.kubernetes.io/component=controller \
		--timeout=120s
	@echo "NGINX Ingress Controller deployed!"

k8s-ingress-down:  ## Uninstall NGINX Ingress Controller
	-helm uninstall ingress-nginx -n ingress-nginx

k8s-build:  ## Build all service Docker images
	@for svc in $(SERVICES); do \
		echo "Building $$svc..."; \
		docker build -t redyx/$$svc-service:dev -f deploy/docker/Dockerfile --build-arg SERVICE=$$svc . ; \
	done

k8s-load:  ## Load all images into kind cluster
	@for svc in $(SERVICES); do \
		echo "Loading $$svc..."; \
		kind load docker-image redyx/$$svc-service:dev --name $(K8S_CLUSTER) ; \
	done

# External images used by data stores, monitoring, and init containers
EXTERNAL_IMAGES := \
    postgres:18-alpine \
    redis:8-alpine \
    scylladb/scylla:2026.1 \
    apache/kafka:3.7.2 \
    getmeili/meilisearch:v1.41 \
    minio/minio:latest \
    minio/mc:latest \
    prom/prometheus:v2.51.0 \
    grafana/grafana:10.4.2 \
    grafana/loki:3.3.2 \
    grafana/promtail:2.9.5 \
    jaegertracing/all-in-one:1.55 \
    envoyproxy/envoy:v1.37.0 \
    busybox

IMAGES_CACHE_DIR ?= $(HOME)/.redyx-images

k8s-pull:  ## Download missing external images to cache, then load all into kind
	@mkdir -p $(IMAGES_CACHE_DIR)
	@echo "Downloading missing images to $(IMAGES_CACHE_DIR)..."
	@for img in $(EXTERNAL_IMAGES); do \
		fname=$(IMAGES_CACHE_DIR)/$$(echo $$img | tr '/:' '_').tar; \
		if [ ! -f $$fname ]; then \
			echo "  Downloading $$img..."; \
			skopeo copy --override-arch amd64 --override-os linux \
				docker://$$img docker-archive:$$fname:$$img & \
		else \
			echo "  $$img already cached, skipping."; \
		fi; \
	done; wait
	@echo "Loading images into kind cluster..."
	@for img in $(EXTERNAL_IMAGES); do \
		fname=$(IMAGES_CACHE_DIR)/$$(echo $$img | tr '/:' '_').tar; \
		kind load image-archive $$fname --name $(K8S_CLUSTER); \
	done
	@echo "All external images loaded into kind."

k8s-data:  ## Deploy data stores (PostgreSQL, Redis, ScyllaDB, Kafka, Meilisearch, MinIO)
	@echo "Deploying PostgreSQL..."
	kubectl apply -f deploy/k8s/data/postgresql.yaml
	@echo "Deploying Redis..."
	kubectl apply -f deploy/k8s/data/redis.yaml
	@echo "Deploying ScyllaDB..."
	kubectl apply -f deploy/k8s/data/scylladb.yaml
	@echo "Deploying Kafka..."
	kubectl apply -f deploy/k8s/data/kafka.yaml
	@echo "Deploying Meilisearch..."
	kubectl apply -f deploy/k8s/data/meilisearch.yaml
	@echo "Deploying MinIO..."
	kubectl apply -f deploy/k8s/data/minio.yaml
	@echo "Waiting for data stores to be ready..."
	-kubectl wait --for=condition=ready pod -l app=postgresql -n $(K8S_NAMESPACE_DATA) --timeout=5m
	-kubectl wait --for=condition=ready pod -l app=redis -n $(K8S_NAMESPACE_DATA) --timeout=5m
	-kubectl wait --for=condition=ready pod -l app=scylladb -n $(K8S_NAMESPACE_DATA) --timeout=10m
	-kubectl wait --for=condition=ready pod -l app=kafka -n $(K8S_NAMESPACE_DATA) --timeout=5m
	-kubectl wait --for=condition=ready pod -l app=meilisearch -n $(K8S_NAMESPACE_DATA) --timeout=5m
	-kubectl wait --for=condition=ready pod -l app=minio -n $(K8S_NAMESPACE_DATA) --timeout=5m
	@echo "Data stores deployed!"

k8s-data-down:  ## Uninstall data stores
	-kubectl delete -f deploy/k8s/data/minio.yaml --ignore-not-found
	-kubectl delete -f deploy/k8s/data/meilisearch.yaml --ignore-not-found
	-kubectl delete -f deploy/k8s/data/kafka.yaml --ignore-not-found
	-kubectl delete -f deploy/k8s/data/scylladb.yaml --ignore-not-found
	-kubectl delete -f deploy/k8s/data/redis.yaml --ignore-not-found
	-kubectl delete -f deploy/k8s/data/postgresql.yaml --ignore-not-found
	-kubectl delete pvc -l app=postgresql -n $(K8S_NAMESPACE_DATA) --ignore-not-found
	-kubectl delete pvc -l app=redis -n $(K8S_NAMESPACE_DATA) --ignore-not-found
	-kubectl delete pvc -l app=scylladb -n $(K8S_NAMESPACE_DATA) --ignore-not-found
	-kubectl delete pvc -l app=kafka -n $(K8S_NAMESPACE_DATA) --ignore-not-found
	-kubectl delete pvc -l app=meilisearch -n $(K8S_NAMESPACE_DATA) --ignore-not-found
	-kubectl delete pvc -l app=minio -n $(K8S_NAMESPACE_DATA) --ignore-not-found

k8s-monitoring:  ## Deploy observability stack (Prometheus, Grafana, Loki, Jaeger)
	@echo "Deploying observability stack..."
	kubectl apply -k deploy/k8s/monitoring
	@echo "Waiting for monitoring stack to be ready..."
	-kubectl wait --for=condition=ready pod -l app=prometheus -n $(K8S_NAMESPACE_MON) --timeout=2m
	-kubectl wait --for=condition=ready pod -l app=loki -n $(K8S_NAMESPACE_MON) --timeout=2m
	-kubectl wait --for=condition=ready pod -l app=jaeger -n $(K8S_NAMESPACE_MON) --timeout=2m
	-kubectl wait --for=condition=ready pod -l app=grafana -n $(K8S_NAMESPACE_MON) --timeout=2m
	@echo "Observability stack deployed!"

k8s-monitoring-down:  ## Uninstall observability stack
	-kubectl delete -k deploy/k8s/monitoring --ignore-not-found
	-kubectl delete pvc -l app=prometheus -n $(K8S_NAMESPACE_MON) --ignore-not-found
	-kubectl delete pvc -l app=loki -n $(K8S_NAMESPACE_MON) --ignore-not-found
	-kubectl delete pvc -l app=grafana -n $(K8S_NAMESPACE_MON) --ignore-not-found

k8s-app:  ## Deploy all microservices via Helm
	@echo "Copying Envoy config files..."
	@mkdir -p deploy/k8s/charts/redyx-services/files
	@cp deploy/envoy/envoy-k8s.yaml deploy/k8s/charts/redyx-services/files/
	@cp deploy/envoy/proto.pb deploy/k8s/charts/redyx-services/files/
	helm upgrade --install redyx-app deploy/k8s/charts/redyx-services \
		-n $(K8S_NAMESPACE_APP) \
		-f deploy/k8s/charts/redyx-services/values-dev.yaml

k8s-up: k8s-create k8s-ingress k8s-storage k8s-build k8s-load k8s-data k8s-monitoring k8s-app  ## Full K8s stack deployment
	@echo ""
	@$(MAKE) k8s-urls

k8s-down:  ## Uninstall Helm releases and delete cluster
	-helm uninstall redyx-app -n $(K8S_NAMESPACE_APP)
	-$(MAKE) k8s-monitoring-down
	-$(MAKE) k8s-data-down
	-$(MAKE) k8s-ingress-down
	kind delete cluster --name $(K8S_CLUSTER)

k8s-logs:  ## Tail logs from all app services
	kubectl logs -n $(K8S_NAMESPACE_APP) -l app.kubernetes.io/instance=redyx-app -f --tail=100

k8s-status:  ## Show cluster status
	@echo "=== Namespaces ===" && kubectl get ns
	@echo "\n=== Pods (app) ===" && kubectl get pods -n $(K8S_NAMESPACE_APP) -o wide
	@echo "\n=== Pods (data) ===" && kubectl get pods -n $(K8S_NAMESPACE_DATA) -o wide
	@echo "\n=== Pods (monitoring) ===" && kubectl get pods -n $(K8S_NAMESPACE_MON) -o wide
	@echo "\n=== Ingresses ===" && kubectl get ingress -A
	@echo "\n=== HPAs ===" && kubectl get hpa -n $(K8S_NAMESPACE_APP)

k8s-urls:  ## Show access URLs (Ingress handles routing)
	@echo ""
	@echo "========================================"
	@echo "  Redyx K8s Local Development URLs"
	@echo "========================================"
	@echo ""
	@echo "API Gateway:  http://localhost:8080/api/v1/"
	@echo "Grafana:      http://localhost:8080/grafana  (admin/admin)"
	@echo "Prometheus:   http://localhost:8080/prometheus"
	@echo "Jaeger:       http://localhost:8080/jaeger"
	@echo ""
	@echo "========================================"

k8s-validate:  ## Run validation script
	./scripts/k8s-validate.sh

.DEFAULT_GOAL := help
