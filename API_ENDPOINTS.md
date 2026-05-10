# API Endpoints Documentation

All endpoints are accessed through the API Gateway at http://localhost:8080

Endpoints marked with [AUTH] require Authorization header with Bearer token.
Token can be obtained from /auth/login endpoint.

---

## Authentication Service (Port: 8090, Direct HTTP)

### POST /auth/register
Register a new user.
- Request: { "username": "string", "password": "string", "role": "string" }
- Response: { "username": "string" }

### POST /auth/login
Login and get JWT token.
- Request: { "username": "string", "password": "string" }
- Response: { "token": "string" }

### GET /auth/me
Get current user claims from token.
- Request: Authorization: Bearer <token>
- Response: { "claims": { "user_id": "string", "role": "string", "exp": number } }

---

## Inventory Service (gRPC: port 50051, HTTP via Gateway: port 8080)

### Stock Management

#### POST /inventory/reserve [AUTH]
Reserve stock for an order (ACID transaction with distributed lock).
- gRPC: ReserveStock
- Request: { "order_id": "string", "items": [{ "sku": "string", "warehouse_id": "string", "qty": number }] }
- Response: { "success": boolean, "reserved_items": [...] }

#### POST /inventory/release [AUTH]
Release reserved stock for a cancelled order.
- gRPC: ReleaseStock
- Request: { "order_id": "string" }
- Response: { "success": boolean, "released_items": [...] }

#### POST /inventory/confirm [AUTH]
Confirm stock deduction after payment (permanent).
- gRPC: ConfirmStockDeduction
- Request: { "order_id": "string" }
- Response: { "success": boolean, "deducted_items": [...] }

#### GET /inventory/sku?sku=SKU-001 [AUTH]
Get current stock level for a SKU across all warehouses.
- gRPC: GetStockBySKU
- Response: { "sku": "string", "locations": [{ "warehouse_id": "string", "total": number, "available": number }] }

#### GET /inventory/warehouse/stocks?warehouse_id=UUID [AUTH]
Get all stock items in a specific warehouse.
- gRPC: ListStocksByWarehouse
- Response: { "warehouse_id": "string", "items": [{ "sku": "string", "total": number, "reserved": number }] }

#### GET /inventory/low?limit=10 [AUTH]
Get items below safety stock level.
- gRPC: GetLowStockItems
- Response: { "items": [{ "sku": "string", "current": number, "safety_level": number, "warehouse_id": "string" }] }

### Stock Operations

#### POST /inventory/receipt [AUTH]
Add stock from receipts/purchase orders.
- gRPC: AddStockReceipt
- Request: { "items": [{ "sku": "string", "warehouse_id": "string", "qty": number }] }
- Response: { "results": [{ "sku": "string", "warehouse_id": "string", "success": boolean }] }

#### POST /inventory/transfer [AUTH]
Transfer stock between warehouses.
- gRPC: TransferStock
- Request: { "sku": "string", "from_warehouse": "string", "to_warehouse": "string", "qty": number }
- Response: { "success": boolean, "qty_transferred": number }

#### POST /inventory/safety [AUTH]
Update safety stock level threshold.
- gRPC: UpdateSafetyStockLevel
- Request: { "sku": "string", "level": number }
- Response: { "success": boolean }

### Warehouse Management

#### GET /inventory/warehouses [AUTH]
List all registered warehouses.
- gRPC: ListWarehouses
- Response: { "warehouses": [{ "id": "string", "name": "string", "location": "string", "is_active": boolean }] }

#### POST /inventory/warehouse [AUTH]
Create a new warehouse.
- gRPC: CreateWarehouse
- Request: { "name": "string", "location": "string" }
- Response: { "warehouse_id": "string", "name": "string" }

#### PUT /inventory/warehouse [AUTH]
Update warehouse information.
- gRPC: UpdateWarehouseInfo
- Request: { "id": "string", "name": "string", "location": "string", "is_active": boolean }
- Response: { "success": boolean }

---

## Catalog Service (gRPC: port 50052, HTTP via Gateway: port 8080)

#### POST /catalog/products [AUTH]
Create a new product.
- gRPC: CreateProduct
- Request: { "sku": "string", "name": "string", "description": "string", "price": number }
- Response: { "product_id": "string", "sku": "string", "name": "string" }

#### GET /catalog/products?product_id=UUID [AUTH]
Get product details.
- gRPC: GetProduct
- Response: { "product_id": "string", "sku": "string", "name": "string", "description": "string", "price": number }

#### GET /catalog/search?q=query [AUTH]
Search products by name/description.
- gRPC: SearchProducts
- Response: { "products": [{ "product_id": "string", "sku": "string", "name": "string", "price": number }] }

#### POST /catalog/price [AUTH]
Update product price.
- gRPC: UpdatePrice
- Request: { "product_id": "string", "new_price": number }
- Response: { "success": boolean, "old_price": number, "new_price": number }

#### POST /catalog/bulk [AUTH]
Get multiple products by IDs.
- gRPC: BulkGetProducts
- Request: { "product_ids": ["string"] }
- Response: { "products": [{ "product_id": "string", "sku": "string", "name": "string", "price": number }] }

---

## Order Service (gRPC: port 50053, HTTP via Gateway: port 8080)

#### POST /orders [AUTH]
Create a new order.
- gRPC: CreateOrder
- Request: { "customer_id": "string", "items": [{ "product_id": "string", "qty": number, "price": number }] }
- Response: { "order_id": "string", "status": "pending", "total": number }

#### GET /orders?order_id=UUID [AUTH]
Get order details.
- gRPC: GetOrder
- Response: { "order_id": "string", "customer_id": "string", "status": "string", "items": [...], "total": number }

#### POST /orders/cancel [AUTH]
Cancel an order.
- gRPC: CancelOrder
- Request: { "order_id": "string" }
- Response: { "success": boolean, "order_id": "string", "status": "cancelled" }

#### POST /orders/status [AUTH]
Update order status.
- gRPC: UpdateStatus
- Request: { "order_id": "string", "new_status": "string" }
- Response: { "success": boolean, "order_id": "string", "status": "string" }

#### POST /orders/calculate [AUTH]
Calculate order total with taxes/discounts.
- gRPC: CalculateTotal
- Request: { "items": [{ "product_id": "string", "qty": number, "price": number }] }
- Response: { "subtotal": number, "tax": number, "total": number }

#### POST /orders/bulk [AUTH]
Get multiple orders by IDs.
- gRPC: BulkGetOrders
- Request: { "order_ids": ["string"] }
- Response: { "orders": [{ "order_id": "string", "status": "string", "total": number }] }

---

## Notification Service (gRPC: port 50054, HTTP via Gateway: port 8080)

#### POST /notifications/email [AUTH]
Send email notification.
- gRPC: SendEmail
- Request: { "to": "string", "subject": "string", "body": "string" }
- Response: { "message_id": "string", "sent": boolean }

#### POST /notifications/order-confirmation [AUTH]
Send order confirmation email.
- gRPC: SendOrderConfirmation
- Request: { "order_id": "string", "customer_email": "string", "order_details": "string" }
- Response: { "message_id": "string", "sent": boolean }

#### POST /notifications/stock-alert [AUTH]
Send stock alert notification.
- gRPC: SendStockAlert
- Request: { "sku": "string", "warehouse_id": "string", "current_qty": number, "threshold": number }
- Response: { "message_id": "string", "sent": boolean }

---

## Infrastructure & Observability

### Health Checks (No Auth Required)
- GET /healthz - Basic health check (returns 200 OK)
- GET /readyz - Readiness check (returns 200 when all dependencies ready)

### Metrics
- GET /metrics - Prometheus metrics endpoint (no auth required)

---

## Authentication

All endpoints marked with [AUTH] require an Authorization header:
```
Authorization: Bearer <jwt_token>
```

Tokens are obtained via /auth/login and are valid for 24 hours.

Default test credentials:
- Username: demo
- Password: pass
- Role: admin

---

## Service Architecture

Services communicate via:
- gRPC for inter-service communication
- NATS message queue for async events
- PostgreSQL for persistent storage
- Redis for caching and distributed locks

Database Details:
- Inventory: inventory_db (port 5432)
- Catalog: catalog_db (port 5433)
- Order: order_db (port 5434)
- Identity: identity_db (port 5435)

Cache:
- Redis: localhost:6379

Message Queue:
- NATS: localhost:4222
