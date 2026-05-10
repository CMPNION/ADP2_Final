package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/cmpnion/adp-final/services/inventory/internal/domain/entities"
)

func (s *ReserveService) AddStockReceipt(ctx context.Context, sku string, warehouseID string, qty int64, receiptID string) error {
	if qty <= 0 {
		return fmt.Errorf("invalid qty")
	}
	key := fmt.Sprintf("lock:stock:%s:%s", sku, warehouseID)
	ok, err := s.locker.Lock(ctx, key, 8*time.Second)
	if err != nil || !ok {
		return fmt.Errorf("failed to lock: %w", err)
	}
	defer s.locker.Unlock(ctx, key)

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	ps, err := s.repo.GetProductStockForUpdate(ctx, tx, sku, warehouseID)
	if err != nil {
		return err
	}
	if ps == nil {
		ps = &entities.ProductStock{ID: uuid.New().String(), SKU: sku, WarehouseID: warehouseID, TotalQuantity: qty, ReservedQuantity: 0, SafetyStockLevel: 0, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	} else {
		ps.TotalQuantity += qty
	}
	if err := s.repo.UpsertProductStock(ctx, tx, ps); err != nil {
		return err
	}

	mov := &entities.StockMovement{ID: uuid.New().String(), SKU: sku, WarehouseID: warehouseID, Type: entities.In, Quantity: qty, ReferenceID: receiptID, CreatedAt: time.Now()}
	if err := s.repo.CreateMovement(ctx, tx, mov); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	_ = s.cache.DeleteStockSnapshot(ctx, sku)
	// publish event
	evt := map[string]interface{}{"sku": sku, "warehouse_id": warehouseID, "quantity": qty}
	if payload, e := json.Marshal(evt); e == nil {
		_ = s.publisher.Publish("inventory.stock.received", payload)
	}
	return nil
}
