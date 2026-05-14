#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cp "$ROOT_DIR/services/catalog-service/proto/catalog.proto" "$ROOT_DIR/proto/catalog.proto"
cp "$ROOT_DIR/services/order-service/proto/order.proto" "$ROOT_DIR/proto/order.proto"
cp "$ROOT_DIR/services/notification-service/proto/notification.proto" "$ROOT_DIR/proto/notification.proto"
cp "$ROOT_DIR/services/inventory/proto/inventory.proto" "$ROOT_DIR/proto/inventory.proto"
cp "$ROOT_DIR/services/inventory/proto/events.proto" "$ROOT_DIR/proto/events.proto"

cd "$ROOT_DIR/proto"
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  catalog.proto order.proto inventory.proto notification.proto events.proto

mkdir -p catalog order inventory notification events
mv -f catalog.pb.go catalog_grpc.pb.go catalog/
mv -f order.pb.go order_grpc.pb.go order/
mv -f inventory.pb.go inventory_grpc.pb.go inventory/
mv -f notification.pb.go notification_grpc.pb.go notification/
mv -f events.pb.go events/

echo "Service proto files synchronized to root proto module and regenerated."
