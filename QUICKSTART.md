# Quick Start Guide

## Prerequisites

- Docker and Docker Compose installed
- Go 1.25+ (for local development)
- PostgreSQL client tools (optional, for manual queries)
- curl or Postman (for API testing)

## Running the System

### 1. Start All Services

```bash
cd /Users/cmpnion/backend/ADP-Final
docker compose up -d --build
```

This starts:
- 6 microservices (API Gateway, Inventory, Notification, Catalog, Order, Auth)
- 5 PostgreSQL databases (one per service)
- Redis cache server
- NATS message broker
- Frontend (if running separately)

### 2. Verify Services are Running

```bash
# Check all containers are healthy
docker compose ps

# Check API Gateway is responding
curl http://localhost:8080/healthz
```

Expected response: `{"status":"ok"}`

## Basic Workflow

### 1. Register User

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john_doe",
    "password": "securepass123"
  }'
```

Response:
```json
{
  "username": "john_doe"
}
```

### 2. Login

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "john_doe",
    "password": "securepass123"
  }'
```

Response:
```json
{
  "token": "eyJ0eXAiOiJKV1QiLCJhbGc..."
}
```

Save the token for subsequent requests:
```bash
TOKEN="eyJ0eXAiOiJKV1QiLCJhbGc..."
```

### 3. View Available Warehouses

```bash
curl http://localhost:8080/inventory/warehouses \
  -H "Authorization: Bearer $TOKEN"
```

### 4. Check Stock Levels

```bash
curl "http://localhost:8080/inventory/sku?sku=SKU001" \
  -H "Authorization: Bearer $TOKEN"
```

### 5. Create an Order

```bash
curl -X POST http://localhost:8080/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "john_doe",
    "items": [
      {
        "sku": "SKU001",
        "qty": 5
      }
    ]
  }'
```

### 6. Reserve Stock

```bash
curl -X POST http://localhost:8080/inventory/reserve \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "order_id": "ORD-001",
    "items": [
      {
        "sku": "SKU001",
        "warehouse_id": "550e8400-e29b-41d4-a716-446655440000",
        "qty": 5
      }
    ]
  }'
```

### 7. Send Email Notification

```bash
curl -X POST http://localhost:8080/notifications/email \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "customer@example.com",
    "subject": "Order Confirmation",
    "body": "Your order has been placed successfully!"
  }'
```

## API Endpoints Summary

| Service | Method | Endpoint | Purpose |
|---------|--------|----------|---------|
| Auth | POST | /auth/register | Register new user |
| Auth | POST | /auth/login | Login and get token |
| Auth | GET | /auth/me | Get current user info |
| Inventory | GET | /inventory/warehouses | List all warehouses |
| Inventory | GET | /inventory/sku?sku=X | Get stock for SKU |
| Inventory | POST | /inventory/reserve | Reserve stock for order |
| Inventory | POST | /inventory/release | Release reserved stock |
| Inventory | POST | /inventory/confirm | Confirm stock deduction |
| Inventory | GET | /inventory/low | Get low stock items |
| Inventory | POST | /inventory/receipt | Add new stock |
| Inventory | POST | /inventory/transfer | Transfer between warehouses |
| Inventory | POST | /inventory/safety | Update safety stock level |
| Inventory | POST | /inventory/warehouse | Create new warehouse |
| Order | POST | /orders | Create order |
| Order | GET | /orders?id=X | Get order details |
| Order | POST | /orders/cancel | Cancel order |
| Order | POST | /orders/status | Update order status |
| Order | POST | /orders/calculate | Calculate order total |
| Catalog | GET | /catalog/search?q=X | Search products |
| Catalog | GET | /catalog/products | List products |
| Catalog | POST | /catalog/price | Update product price |
| Notification | POST | /notifications/email | Send email |
| Notification | POST | /notifications/order-confirmation | Send order confirmation |
| Notification | POST | /notifications/stock-alert | Send stock alert |

## Database Access

### Connect to Inventory Database

```bash
# From your host machine
psql -h localhost -U postgres -d inventory_db -W

# Password: postgres (default)
```

Useful queries:
```sql
-- View warehouses
SELECT * FROM warehouses;

-- View product stock
SELECT * FROM product_stocks;

-- View stock reservations
SELECT * FROM stock_reservations;

-- View stock movements
SELECT * FROM stock_movements;
```

## Logs

### View API Gateway Logs

```bash
docker logs adp-final-api-gateway-1 -f
```

### View Inventory Service Logs

```bash
docker logs adp-final-inventory-service-1 -f
```

### View All Service Logs

```bash
docker compose logs -f
```

## Troubleshooting

### Services Won't Start

1. Check Docker is running:
```bash
docker --version
```

2. Check ports are available:
```bash
lsof -i :8080  # API Gateway
lsof -i :5432  # PostgreSQL
lsof -i :6379  # Redis
```

3. View build logs:
```bash
docker compose logs --tail 100
```

### Database Connection Errors

1. Verify PostgreSQL is running:
```bash
docker ps | grep postgres
```

2. Check database exists:
```bash
docker compose exec postgres_inventory psql -U postgres -l
```

### JWT Token Errors

1. Ensure token is valid and not expired:
```bash
curl http://localhost:8080/auth/me \
  -H "Authorization: Bearer $TOKEN"
```

2. If token is invalid, login again to get a new one

### Mailgun Email Issues

1. Verify environment variables are set:
```bash
docker compose exec api-gateway env | grep MAILGUN
```

2. Check Mailgun credentials are correct in .env file

## Performance Testing

### Load Test Inventory Service

```bash
# Install hey (HTTP load generator)
go install github.com/rakyll/hey@latest

# Test list warehouses
hey -n 1000 -c 100 \
  -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/inventory/warehouses
```

### Benchmark Stock Reservation

```bash
# Create multiple orders concurrently
for i in {1..100}; do
  curl -X POST http://localhost:8080/inventory/reserve \
    -H "Authorization: Bearer $TOKEN" \
    -H "Content-Type: application/json" \
    -d "{
      \"order_id\": \"ORD-$i\",
      \"items\": [{
        \"sku\": \"SKU001\",
        \"warehouse_id\": \"550e8400-e29b-41d4-a716-446655440000\",
        \"qty\": 1
      }]
    }" &
done
wait
```

## Stopping Services

```bash
# Stop all services
docker compose down

# Stop and remove volumes (clears databases)
docker compose down -v
```

## Next Steps

1. Read the [API Documentation](./API_DOCUMENTATION.md) for complete endpoint details
2. Review the [System Architecture](./ARCHITECTURE.md) for design details
3. Check service logs to understand request flow
4. Modify .env file to customize configuration
5. Integrate with your frontend application

## Support

For issues or questions:
1. Check the logs first
2. Verify all services are running with `docker compose ps`
3. Ensure environment variables are correctly set
4. Review the API Documentation for endpoint usage

---

Last Updated: May 10, 2026
