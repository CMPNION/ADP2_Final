# Omnichannel Inventory Management System

Production-ready microservices architecture for distributed inventory management with gRPC, PostgreSQL, Redis, and NATS.

## Project Structure

```
.
├── proto/                          # Shared generated Go protobuf module
│   ├── *.proto                     # Synced from service-local proto files
│   ├── sync_service_protos.sh      # Sync + regenerate helper
│   └── {catalog,order,...}/        # Generated .pb.go files
│
├── services/
│   ├── inventory/                  # Inventory Microservice
│   │   ├── proto/                  # inventory.proto + events.proto (source of truth)
│   │   ├── internal/
│   │   │   ├── domain/             # Entities & interfaces
│   │   │   ├── application/        # Business logic (usecases)
│   │   │   ├── infra/              # Repository, Cache, Publisher
│   │   │   ├── delivery/           # gRPC handlers
│   │   │   └── migrations/         # SQL migrations
│   │   └── cmd/                    # Service entrypoint
│   │
│   ├── catalog-service/            # Catalog Microservice
│   │   ├── proto/                  # catalog.proto (source of truth)
│   │   └── cmd/
│   │
│   ├── order-service/              # Order Microservice
│   │   ├── proto/                  # order.proto (source of truth)
│   │   └── cmd/
│   │
│   ├── notification-service/       # Notification Microservice
│   │   ├── proto/                  # notification.proto (+ events.proto copy)
│   │   └── cmd/
│   │
│   └── auth-service/               # Authentication Service (HTTP)
│       └── cmd/
│
├── api-gateway/                    # REST → gRPC API Gateway
│   ├── internal/app/
│   │   └── server.go               # REST handlers & routing
│   └── cmd/
│
├── frontend/                       # React UI
│   └── src/App.js
│
├── docker-compose.yml              # Service orchestration
├── Dockerfile                      # Multi-service build
└── API_ENDPOINTS.md               # Complete endpoint reference

```

## Services

### Inventory Service (gRPC: 50051)
Core stock management with ACID transactions, distributed locking, and real-time caching.

Endpoints:
- ReserveStock - Atomic reservation with FOR UPDATE lock
- ReleaseStock - Unreserve on order cancellation
- ConfirmStockDeduction - Permanent deduction after payment
- GetStockBySKU - Availability across warehouses
- ListStocksByWarehouse - Per-location breakdown
- AddStockReceipt - Supply chain intake
- TransferStock - Inter-warehouse movement
- UpdateSafetyStockLevel - Threshold configuration
- GetLowStockItems - Replenishment alerts
- CreateWarehouse - New location registration
- UpdateWarehouseInfo - Warehouse metadata
- ListWarehouses - All locations

### Catalog Service (gRPC: 50052)
Product information and pricing management.

Endpoints:
- CreateProduct
- GetProduct
- SearchProducts
- UpdatePrice
- BulkGetProducts
- DeleteProduct
- ListProducts
- UpsertProduct
- BatchUpdatePrice
- GetProductsByPriceRange
- AdjustPriceByPercent
- GetCatalogStats

### Order Service (gRPC: 50053)
Order lifecycle and cart calculations.

Endpoints:
- CreateOrder
- GetOrder
- CancelOrder
- UpdateStatus
- CalculateTotal
- BulkGetOrders
- ListOrdersByUser
- ListOrdersByStatus
- ConfirmOrder
- MarkOrderPaid
- ShipOrder
- GetOrderStats

### Notification Service (gRPC: 50054)
Email and alert notifications via NATS events.

Endpoints:
- SendEmail
- SendOrderConfirmation
- SendStockAlert

### Auth Service (HTTP: 8090)
JWT token issuance and user management.

Endpoints:
- POST /auth/register
- POST /auth/login
- GET /auth/me

### API Gateway (HTTP: 8080)
REST → gRPC proxy with auth middleware, rate limiting, and metrics.

All service endpoints exposed as HTTP endpoints.

## Database Schema

### Inventory DB (PostgreSQL 5432)
- warehouses - Warehouse locations
- product_stocks - Stock per SKU/warehouse with reservation tracking
- stock_reservations - Order-level reservations (PENDING/CONFIRMED/RELEASED)
- stock_movements - Audit trail of all stock changes

Indexes on SKU, available qty, and order_id for fast lookups.

### Catalog DB (PostgreSQL 5433)
- products - Product master data

### Order DB (PostgreSQL 5434)
- orders - Order records with status tracking

### Identity DB (PostgreSQL 5435)
- users - Authentication records

## Technology Stack

Runtime: Go 1.25
Protocol: gRPC (proto3)
Transport: HTTP/2 (gRPC), HTTP/1.1 (REST)
Database: PostgreSQL 15
Cache: Redis 7
Queue: NATS 2.x
Container: Docker & Docker Compose
Observability: Prometheus metrics

## Key Features

ACID Transactions - All stock deductions wrapped in DB transactions with SELECT FOR UPDATE
Distributed Locking - Redis SetNX with TTL for concurrent reservation safety
Caching - Real-time stock snapshots in Redis
Event Driven - NATS pub/sub for order.created and inventory.stock.low
Metrics - Prometheus counter/histogram for all gRPC and HTTP operations
Circuit Ready - Separate health/readiness endpoints
Clean Architecture - Domain → Usecase → Infra → Delivery layer separation

## Getting Started

### Prerequisites
- Docker & Docker Compose
- Go 1.25 (local development)
- PostgreSQL 15, Redis 7, NATS (or use provided docker-compose)

### Quick Start

1. Start services
   docker compose up -d --build

2. Register & Login
   - Frontend: http://localhost:3000
   - Username: demo, Password: pass
   - Or use curl for auth service on http://localhost:8090

3. API Gateway available at http://localhost:8080
   All endpoints require Authorization: Bearer <jwt_token>

4. View metrics
   http://localhost:8080/metrics

### Manual Setup (Development)

1. Start infrastructure
   docker compose up -d postgres_inventory redis nats

2. Apply migrations
   cat services/inventory/migrations/002_schema.sql | psql -U postgres -d inventory_db

3. Build and run services
   go build ./services/inventory/cmd
   go build ./services/catalog-service/cmd
   go build ./services/order-service/cmd
   go build ./services/notification-service/cmd
   go build ./services/auth-service/cmd

   Set env vars:
   export DATABASE_URL=postgres://postgres:pass@localhost:5432/inventory_db?sslmode=disable
   export REDIS_ADDR=localhost:6379
   export NATS_URL=nats://localhost:4222
   export JWT_SECRET=dev-jwt-secret

## Configuration

Services configured via environment variables:

Inventory Service:
- DATABASE_URL=postgres://... (required)
- REDIS_ADDR=host:port (default localhost:6379)
- NATS_URL=nats://host:port (default nats://localhost:4222)
- GRPC_ADDR=:50051 (default)
- METRICS_ADDR=:9090 (default)

Auth Service:
- JWT_SECRET=secret (required, default dev-jwt-secret)
- HTTP_ADDR=:8090 (default)

API Gateway:
- HTTP_ADDR=:8080 (default)
- INVENTORY_GRPC_ADDR=inventory-service:50051
- JWT_SECRET=secret (same as auth-service)
- RATE_LIMIT_PER_MINUTE=120 (default)

## Testing

Frontend available at http://localhost:3000

Available test data created during initialization:
- Warehouses: A, B, C (New York, Los Angeles, Chicago)
- SKUs: SKU-001, SKU-002, SKU-003

Test Flows:
1. Login with demo/pass
2. View inventory (SKU lookup, low stock)
3. Make reservations (with qty constraints)
4. Manage warehouses (create, view)
5. Transfer stock between locations
6. Manage products and orders (when proxies are enabled)

## API Reference

See API_ENDPOINTS.md for complete endpoint documentation with request/response examples.

## Observability

Prometheus Metrics - /metrics endpoint exposes:
- api_gateway_http_requests_total - Request counter by method/path/status
- api_gateway_http_request_duration_seconds - Request latency histogram
- grpc interceptors measure latency for all gRPC calls

Logs - Structured JSON logs with correlation IDs from gateway

Health Checks:
- /healthz - Basic health (200 OK if running)
- /readyz - Readiness (checks dependencies)

## Production Considerations

1. Database: Use managed PostgreSQL (AWS RDS, GCP Cloud SQL)
2. Cache: Use managed Redis (AWS ElastiCache, GCP Memorystore)
3. Queue: Use managed NATS (NATS Cloud) or deploy clustered NATS
4. Auth: Integrate with Keycloak, Auth0, or proper OAuth2 provider
5. TLS: Enable HTTPS on API Gateway and service-to-service gRPC
6. Secrets: Use HashiCorp Vault or cloud secret manager
7. Monitoring: Enable OpenTelemetry traces to Grafana/Jaeger
8. Scaling: Use Kubernetes with HPA for autoscaling

## Development Notes

Clean Architecture Pattern:
- domain/: Interfaces & entities (database agnostic)
- application/: Business logic & usecases (framework independent)
- infra/: Concrete implementations (DB, cache, queue)
- delivery/: Transport layer (gRPC, HTTP)

Redis Locks Pattern:
- SetNX with expiration for distributed mutual exclusion
- Prevents overselling in concurrent scenarios

NATS Subscriptions:
- order.created → triggers inventory.reserve
- inventory.stock.low → published on threshold breach

Testing:
- Unit tests in usecases (mock repository)
- Integration tests for postgres repository
- Note: All tests omitted for faster demo

## Future Enhancements

- OpenTelemetry spans and traces
- User persistence in PostgreSQL (auth-service)
- Product variants and bulk pricing
- Multi-currency support
- Inventory aging and obsolescence tracking
- Supplier integration for auto-replenishment
- Machine learning for demand forecasting
