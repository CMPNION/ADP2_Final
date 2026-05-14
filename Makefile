SHELL := /bin/bash

.PHONY: help up down rebuild clean reset migrate migrate-inventory migrate-catalog migrate-order migrate-identity proto-sync test-backend test-frontend test-all bootstrap

help:
	@echo "Targets:"
	@echo "  make up                - docker compose up -d --build"
	@echo "  make down              - docker compose down"
	@echo "  make rebuild           - docker compose build --no-cache"
	@echo "  make clean             - docker compose down -v --remove-orphans"
	@echo "  make reset             - clean + up"
	@echo "  make migrate           - run all available migrations"
	@echo "  make proto-sync        - sync service proto files and regenerate pb.go"
	@echo "  make test-backend      - run Go tests"
	@echo "  make test-frontend     - run frontend tests"
	@echo "  make test-all          - backend + frontend tests"
	@echo "  make bootstrap         - full setup from zero: clean, up, migrate, test-all"

up:
	docker compose up -d --build

down:
	docker compose down

rebuild:
	docker compose build --no-cache

clean:
	docker compose down -v --remove-orphans

reset: clean up

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

bootstrap: clean up
	@echo "Waiting for databases to become healthy..."
	sleep 10
	$(MAKE) migrate
	$(MAKE) test-all
