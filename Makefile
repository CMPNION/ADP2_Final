SHELL := /bin/bash

TF_DIR ?= terraform
K8S_DIR ?= k8s
LOADTEST_DIR ?= loadtesting
TARGET_URL ?= http://localhost:8080

.PHONY: help up down rebuild clean reset wait-for-databases migrate migrate-inventory migrate-catalog migrate-order migrate-identity proto-sync test-backend test-frontend test-all stack bootstrap terraform-init terraform-fmt terraform-validate terraform-apply k8s-apply monitoring-apply platform-up all locust locust-install k8s-destroy terraform-destroy destroy-all

help:
	@echo "Targets:"
	@echo "  make up                - docker compose up -d --build"
	@echo "  make down              - docker compose down"
	@echo "  make rebuild           - docker compose build --no-cache"
	@echo "  make clean             - docker compose down -v --remove-orphans"
	@echo "  make reset             - clean + up"
	@echo "  make stack             - up + wait for DBs + migrate (full local platform)"
	@echo "  make platform-up      - terraform + k8s + monitoring + local stack"
	@echo "  make all               - alias for platform-up"
	@echo "  make locust           - install Locust if needed and run it against TARGET_URL"
	@echo "  make locust-install   - install Locust dependencies"
	@echo "  make migrate           - run all available migrations"
	@echo "  make proto-sync        - sync service proto files and regenerate pb.go"
	@echo "  make test-backend      - run Go tests"
	@echo "  make test-frontend     - run frontend tests"
	@echo "  make test-all          - backend + frontend tests"
	@echo "  make bootstrap         - full setup from zero: clean, up, migrate, test-all"
	@echo ""
	@echo "CLEANUP & DESTROY:"
	@echo "  make k8s-destroy       - delete all Kubernetes manifests (HPA, monitoring, ingress, services)"
	@echo "  make terraform-destroy - destroy Terraform infrastructure"
	@echo "  make destroy-all       - full teardown: k8s + terraform + docker + volumes"

up:
	docker compose up -d --build

down:
	docker compose down

rebuild:
	docker compose build --no-cache

clean:
	docker compose down -v --remove-orphans

reset: clean up

wait-for-databases:
	@echo "Waiting for PostgreSQL containers to become ready..."
	@until docker compose exec -T postgres_inventory pg_isready -U postgres -d inventory_db >/dev/null 2>&1 && \
		docker compose exec -T postgres_catalog pg_isready -U postgres -d catalog_db >/dev/null 2>&1 && \
		docker compose exec -T postgres_order pg_isready -U postgres -d order_db >/dev/null 2>&1 && \
		docker compose exec -T postgres_identity pg_isready -U postgres -d identity_db >/dev/null 2>&1; do \
		sleep 2; \
	done

migrate: migrate-inventory migrate-catalog migrate-order migrate-identity

migrate-inventory:
	@echo "Applying inventory migration..."
	docker compose exec -T postgres_inventory psql -U postgres -d inventory_db < services/inventory/migrations/002_schema.sql

migrate-catalog:
	@echo "Applying catalog migration..."
	docker compose exec -T postgres_catalog psql -U postgres -d catalog_db < services/catalog-service/migrations/001_schema.sql

migrate-order:
	@echo "Applying order migration..."
	docker compose exec -T postgres_order psql -U postgres -d order_db < services/order-service/migrations/001_schema.sql

migrate-identity:
	@echo "Applying identity/auth migration..."
	docker compose exec -T postgres_identity psql -U postgres -d identity_db < services/auth-service/migrations/001_schema.sql

proto-sync:
	./proto/sync_service_protos.sh

test-backend:
	go test ./api-gateway/... ./services/auth-service/... ./services/inventory/... ./services/catalog-service/... ./services/order-service/... ./services/notification-service/... ./proto/...

test-frontend:
	cd frontend && npm ci --silent && CI=true npm test -- --watchAll=false

test-all: test-backend test-frontend

bootstrap: clean stack test-all

stack: up wait-for-databases migrate

terraform-init:
	terraform -chdir=$(TF_DIR) init

terraform-fmt:
	terraform -chdir=$(TF_DIR) fmt -check

terraform-validate: terraform-init
	terraform -chdir=$(TF_DIR) validate

terraform-apply: terraform-validate
	terraform -chdir=$(TF_DIR) apply -auto-approve

k8s-apply:
	kubectl apply -f $(K8S_DIR)/backend/backend.yaml
	kubectl apply -f $(K8S_DIR)/frontend/frontend.yaml
	kubectl apply -f $(K8S_DIR)/ingress/ingress.yaml
	kubectl apply -f $(K8S_DIR)/autoscaling/hpa.yaml

monitoring-apply:
	kubectl apply -f $(K8S_DIR)/monitoring/stack.yaml

platform-up: terraform-fmt terraform-validate up wait-for-databases migrate k8s-apply monitoring-apply

all: platform-up

locust-install:
	python3 -m pip install --user -r $(LOADTEST_DIR)/requirements.txt

locust:
	$(MAKE) locust-install
	python3 -m locust -f $(LOADTEST_DIR)/locustfile.py --host=$(TARGET_URL)

# === CLEANUP & DESTROY TARGETS ===

k8s-destroy:
	@echo "Deleting Kubernetes manifests..."
	-kubectl delete -f $(K8S_DIR)/autoscaling/hpa.yaml 2>/dev/null || true
	-kubectl delete -f $(K8S_DIR)/monitoring/stack.yaml 2>/dev/null || true
	-kubectl delete -f $(K8S_DIR)/ingress/ingress.yaml 2>/dev/null || true
	-kubectl delete -f $(K8S_DIR)/frontend/frontend.yaml 2>/dev/null || true
	-kubectl delete -f $(K8S_DIR)/backend/backend.yaml 2>/dev/null || true
	@echo "Kubernetes manifests deleted."

terraform-destroy:
	@echo "Destroying Terraform infrastructure..."
	terraform -chdir=$(TF_DIR) destroy -auto-approve || true
	@echo "Terraform state cleaned up."

destroy-all: k8s-destroy terraform-destroy clean
	@echo ""
	@echo "✓ Full teardown completed:"
	@echo "  - Kubernetes manifests deleted"
	@echo "  - Terraform infrastructure destroyed"
	@echo "  - Docker containers stopped and volumes removed"
	@echo ""
	@echo "To rebuild: make all"
