# Omnichannel Inventory Management System - API Documentation

## Overview

This is a production-grade omnichannel microservices platform built with Go, featuring gRPC services for inter-service communication and HTTP REST endpoints through the API Gateway for client access.

## System Architecture

- **API Gateway**: Central entry point for all HTTP requests (localhost:8080)
- **Inventory Service**: gRPC service for stock management (localhost:50051)
- **Notification Service**: gRPC service for email notifications via Mailgun (localhost:50054)
- **Catalog Service**: gRPC service for product catalog management (localhost:50052)
- **Order Service**: gRPC service for order management (localhost:50053)
- **Auth Service**: HTTP service for JWT authentication (localhost:8090)
- **PostgreSQL**: Data persistence (multiple databases per service)
- **Redis**: Distributed caching and locking
- **NATS**: Asynchronous event messaging

## Authentication

All protected endpoints require a Bearer token in the Authorization header:

```
Authorization: Bearer <JWT_TOKEN>
```

### Auth Endpoints (HTTP)

#### Register User
- **URL**: `POST /auth/register`
- **Authentication**: None
- **Request Body**:
```json
{
  "username": "string",
  "password": "string",
  "role": "string" (optional)
}
```
- **Response**: `{"username": "string"}`
- **Status Codes**: 201 (Created), 400 (Bad Request), 409 (Conflict)

#### Login
- **URL**: `POST /auth/login`
- **Authentication**: None
- **Request Body**:
```json
{
  "username": "string",
  "password": "string"
}
```
- **Response**: `{"token": "JWT_TOKEN_STRING"}`
- **Status Codes**: 200 (OK), 400 (Bad Request), 401 (Unauthorized)

#### Get Current User
- **URL**: `GET /auth/me`
- **Authentication**: Required (Bearer token)
- **Response**: 
```json
{
  "claims": {
    "user_id": "string",
    "role": "string",
    "exp": number
  }
}
```
- **Status Codes**: 200 (OK), 401 (Unauthorized)

---

## Inventory Service (12 gRPC Endpoints)

All inventory endpoints are exposed through the HTTP API Gateway for REST clients. The underlying gRPC service communicates with PostgreSQL for transaction-safe stock management and Redis for distributed locking.

### HTTP Endpoints (via API Gateway)

#### Reserve Stock
- **URL**: `POST /inventory/reserve`
- **Authentication**: Required
- **Request Body**:
```json
{
  "order_id": "string",
  "items": [
    {
      "sku": "string",
      "warehouse_id": "string",
      "qty": integer
    }
  ]
}
```
- **Response**: `{"message": "string"}`
- **Description**: Atomically reserves items for an order using Redis-based distributed locks and PostgreSQL ACID transactions

#### Release Stock
- **URL**: `POST /inventory/release`
- **Authentication**: Required
- **Request Body**:
```json
{
  "order_id": "string"
}
```
- **Response**: `{"message": "string"}`
- **Description**: Unreserves items if an order is cancelled

#### Confirm Stock Deduction
- **URL**: `POST /inventory/confirm`
- **Authentication**: Required
- **Request Body**:
```json
{
  "order_id": "string"
}
```
- **Response**: `{"message": "string"}`
- **Description**: Permanent deduction after payment is confirmed

#### Get Stock by SKU
- **URL**: `GET /inventory/sku?sku=SKU_VALUE`
- **Authentication**: Required
- **Response**:
```json
{
  "sku": "string",
  "total_available": integer,
  "total_reserved": integer
}
```
- **Description**: Returns current availability across all warehouses

#### List Stocks by Warehouse
- **URL**: `GET /inventory/warehouse/stocks?warehouse_id=WAREHOUSE_ID`
- **Authentication**: Required
- **Response**:
```json
{
  "stocks": [
    {
      "sku": "string",
      "warehouse_id": "string",
      "total_quantity": integer,
      "reserved_quantity": integer,
      "available_quantity": integer,
      "safety_stock_level": integer
    }
  ]
}
```
- **Description**: Breakdown of inventory per location

#### Add Stock Receipt
- **URL**: `POST /inventory/receipt`
- **Authentication**: Required
- **Request Body**:
```json
{
  "sku": "string",
  "warehouse_id": "string",
  "quantity": integer,
  "receipt_id": "string"
}
```
- **Response**: `{"message": "string"}`
- **Description**: Supply chain intake (adding new stock)

#### Transfer Stock
- **URL**: `POST /inventory/transfer`
- **Authentication**: Required
- **Request Body**:
```json
{
  "sku": "string",
  "from_warehouse_id": "string",
  "to_warehouse_id": "string",
  "quantity": integer,
  "reference_id": "string"
}
```
- **Response**: `{"message": "string"}`
- **Description**: Move inventory between warehouses

#### Update Safety Stock Level
- **URL**: `POST /inventory/safety`
- **Authentication**: Required
- **Request Body**:
```json
{
  "sku": "string",
  "warehouse_id": "string",
  "level": integer
}
```
- **Response**: `{"message": "string"}`
- **Description**: Set thresholds for "low stock" alerts

#### Get Low Stock Items
- **URL**: `GET /inventory/low?limit=10`
- **Authentication**: Required
- **Response**:
```json
{
  "stocks": [
    {
      "sku": "string",
      "warehouse_id": "string",
      "total_quantity": integer,
      "reserved_quantity": integer,
      "available_quantity": integer,
      "safety_stock_level": integer
    }
  ]
}
```
- **Description**: List items requiring replenishment

#### Create Warehouse
- **URL**: `POST /inventory/warehouse`
- **Authentication**: Required
- **Request Body**:
```json
{
  "name": "string",
  "location": "string"
}
```
- **Response**: `{"warehouse_id": "string"}`
- **Description**: Register a new storage location

#### Update Warehouse Info
- **URL**: `PUT /inventory/warehouse`
- **Authentication**: Required
- **Request Body**:
```json
{
  "warehouse_id": "string",
  "name": "string",
  "location": "string",
  "is_active": boolean
}
```
- **Response**: `{"message": "string"}`
- **Description**: Edit warehouse metadata

#### List Warehouses
- **URL**: `GET /inventory/warehouses`
- **Authentication**: Required
- **Response**:
```json
{
  "warehouses": [
    {
      "warehouse_id": "string",
      "name": "string",
      "location": "string",
      "is_active": boolean
    }
  ]
}
```
- **Description**: Retrieve all registered locations

### gRPC Service Definition

```protobuf
service InventoryService {
  rpc ReserveStock(ReserveStockRequest) returns (ReserveStockResponse);
  rpc ReleaseStock(ReleaseStockRequest) returns (ReleaseStockResponse);
  rpc ConfirmStockDeduction(ConfirmStockDeductionRequest) returns (ConfirmStockDeductionResponse);
  rpc GetStockBySKU(GetStockBySKURequest) returns (GetStockBySKUResponse);
  rpc ListStocksByWarehouse(ListStocksByWarehouseRequest) returns (ListStocksByWarehouseResponse);
  rpc AddStockReceipt(AddStockReceiptRequest) returns (AddStockReceiptResponse);
  rpc TransferStock(TransferStockRequest) returns (TransferStockResponse);
  rpc UpdateSafetyStockLevel(UpdateSafetyStockLevelRequest) returns (UpdateSafetyStockLevelResponse);
  rpc GetLowStockItems(GetLowStockItemsRequest) returns (GetLowStockItemsResponse);
  rpc CreateWarehouse(CreateWarehouseRequest) returns (CreateWarehouseResponse);
  rpc UpdateWarehouseInfo(UpdateWarehouseInfoRequest) returns (UpdateWarehouseInfoResponse);
  rpc ListWarehouses(ListWarehousesRequest) returns (ListWarehousesResponse);
}
```

---

## Catalog Service (5 gRPC Endpoints)

### HTTP Endpoints (via API Gateway)

#### Get Products
- **URL**: `GET /catalog/products`
- **Authentication**: Required
- **Response**:
```json
{
  "products": []
}
```

#### Search Products
- **URL**: `GET /catalog/search?q=QUERY`
- **Authentication**: Required
- **Response**:
```json
{
  "products": [],
  "query": "string"
}
```
- **Description**: Full-text search across product catalog

#### Update Product Price
- **URL**: `POST /catalog/price`
- **Authentication**: Required
- **Request Body**:
```json
{
  "product_id": "string",
  "price": number
}
```
- **Response**: `{"message": "string"}`

#### Bulk Get Products
- **URL**: `POST /catalog/bulk`
- **Authentication**: Required
- **Request Body**:
```json
{
  "product_ids": ["string"]
}
```
- **Response**: `{"products": []}`

### gRPC Service Definition

```protobuf
service CatalogService {
  rpc CreateProduct(CreateProductRequest) returns (CreateProductResponse);
  rpc GetProduct(GetProductRequest) returns (GetProductResponse);
  rpc SearchProducts(SearchProductsRequest) returns (SearchProductsResponse);
  rpc UpdatePrice(UpdatePriceRequest) returns (UpdatePriceResponse);
  rpc BulkGetProducts(BulkGetProductsRequest) returns (BulkGetProductsResponse);
}
```

---

## Order Service (6 gRPC Endpoints)

### HTTP Endpoints (via API Gateway)

#### Create Order
- **URL**: `POST /orders`
- **Authentication**: Required
- **Request Body**:
```json
{
  "user_id": "string",
  "items": [
    {
      "sku": "string",
      "qty": integer
    }
  ]
}
```
- **Response**: `{"order_id": "string", "status": "pending"}`

#### Get Order
- **URL**: `GET /orders?id=ORDER_ID`
- **Authentication**: Required
- **Response**:
```json
{
  "order_id": "string",
  "status": "string",
  "items": []
}
```

#### Cancel Order
- **URL**: `POST /orders/cancel`
- **Authentication**: Required
- **Request Body**:
```json
{
  "order_id": "string"
}
```
- **Response**: `{"order_id": "string", "status": "cancelled"}`

#### Update Order Status
- **URL**: `POST /orders/status`
- **Authentication**: Required
- **Request Body**:
```json
{
  "order_id": "string",
  "status": "string"
}
```
- **Response**: `{"order_id": "string", "status": "string"}`

#### Calculate Order Total
- **URL**: `POST /orders/calculate`
- **Authentication**: Required
- **Request Body**:
```json
{
  "items": []
}
```
- **Response**: `{"total": number, "tax": number, "subtotal": number}`

#### Bulk Get Orders
- **URL**: `POST /orders/bulk`
- **Authentication**: Required
- **Request Body**:
```json
{
  "order_ids": ["string"]
}
```
- **Response**: `{"orders": []}`

### gRPC Service Definition

```protobuf
service OrderService {
  rpc CreateOrder(CreateOrderRequest) returns (CreateOrderResponse);
  rpc CancelOrder(CancelOrderRequest) returns (CancelOrderResponse);
  rpc GetOrder(GetOrderRequest) returns (GetOrderResponse);
  rpc UpdateStatus(UpdateStatusRequest) returns (UpdateStatusResponse);
  rpc CalculateTotal(CalculateTotalRequest) returns (CalculateTotalResponse);
  rpc BulkGetOrders(BulkGetOrdersRequest) returns (BulkGetOrdersResponse);
}
```

---

## Notification Service (3 gRPC Endpoints)

The Notification Service integrates with Mailgun for email delivery.

### HTTP Endpoints (via API Gateway)

#### Send Email
- **URL**: `POST /notifications/email`
- **Authentication**: Required
- **Request Body**:
```json
{
  "email": "string",
  "subject": "string",
  "body": "string"
}
```
- **Response**: `{"message": "string"}`
- **Description**: Send custom email via Mailgun

#### Send Order Confirmation
- **URL**: `POST /notifications/order-confirmation`
- **Authentication**: Required
- **Request Body**:
```json
{
  "email": "string",
  "order_id": "string"
}
```
- **Response**: `{"message": "string"}`
- **Description**: Send order confirmation email

#### Send Stock Alert
- **URL**: `POST /notifications/stock-alert`
- **Authentication**: Required
- **Request Body**:
```json
{
  "email": "string",
  "sku": "string",
  "message": "string"
}
```
- **Response**: `{"message": "string"}`
- **Description**: Send low stock alert email

### gRPC Service Definition

```protobuf
service NotificationService {
  rpc SendEmail(SendEmailRequest) returns (SendEmailResponse);
  rpc SendOrderConfirmation(SendOrderConfirmationRequest) returns (SendOrderConfirmationResponse);
  rpc SendStockAlert(SendStockAlertRequest) returns (SendStockAlertResponse);
}
```

---

## System Endpoints

#### Health Check
- **URL**: `GET /healthz`
- **Authentication**: None
- **Response**: `ok`

#### Readiness Check
- **URL**: `GET /readyz`
- **Authentication**: None
- **Response**: `{"status": "ready"}`

---

## Total Endpoint Count

- **HTTP Endpoints (via API Gateway)**: 26
  - Auth: 3
  - Inventory: 12
  - Catalog: 4
  - Order: 5
  - Notification: 3
  - System: 2 (health, readiness)

- **gRPC Endpoints**: 26
  - Inventory Service: 12
  - Notification Service: 3
  - Catalog Service: 5
  - Order Service: 6

---

## Technology Stack

- **Language**: Go 1.25
- **RPC Framework**: gRPC with Protocol Buffers (proto3)
- **Database**: PostgreSQL with ACID transactions
- **Caching**: Redis for distributed locking and cache
- **Message Queue**: NATS for asynchronous events
- **Email Service**: Mailgun
- **Authentication**: JWT (HS256)
- **Cryptography**: bcrypt for password hashing
- **Container**: Docker with multi-stage builds

---

## Example Usage

### 1. Register and Login

```bash
# Register
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"user1","password":"pass123"}'

# Login
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"user1","password":"pass123"}'
# Returns: {"token": "eyJ..."}
```

### 2. Reserve Stock

```bash
TOKEN="eyJ..."

curl -X POST http://localhost:8080/inventory/reserve \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": "ORD-001",
    "items": [{
      "sku": "SKU001",
      "warehouse_id": "550e8400-e29b-41d4-a716-446655440000",
      "qty": 5
    }]
  }'
```

### 3. Create Order

```bash
curl -X POST http://localhost:8080/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "user1",
    "items": [{
      "sku": "SKU001",
      "qty": 2
    }]
  }'
```

### 4. Send Email Notification

```bash
curl -X POST http://localhost:8080/notifications/email \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "customer@example.com",
    "subject": "Order Confirmation",
    "body": "Your order has been placed successfully"
  }'
```

---

## Environment Variables

Key environment variables (see .env):

- `JWT_SECRET`: Secret key for JWT signing
- `MAILGUN_API_KEY`: Mailgun API key for email delivery
- `MAILGUN_DOMAIN`: Mailgun sandbox domain
- `MAILGUN_FROM_EMAIL`: Sender email address
- `INVENTORY_DB_URL`: PostgreSQL connection string for inventory service
- `NATS_URL`: NATS server URL for messaging
- `REDIS_ADDR`: Redis server address for caching/locking

---

## Observability

The API Gateway implements:
- Structured JSON logging with request IDs
- Rate limiting middleware
- Authentication middleware
- Prometheus metrics endpoint (if enabled)
- OpenTelemetry tracing compatibility

---

## Error Handling

All errors are returned as JSON with appropriate HTTP status codes:

```json
{
  "error": "error message"
}
```

Common status codes:
- 200: Success
- 201: Created
- 400: Bad Request
- 401: Unauthorized
- 404: Not Found
- 409: Conflict
- 500: Internal Server Error
