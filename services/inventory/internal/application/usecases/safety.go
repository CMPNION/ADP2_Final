package usecases

import (
	"context"
	"encoding/json"
)

func (s *ReserveService) UpdateSafetyStockLevel(ctx context.Context, sku string, warehouseID string, level int64) error {
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
		return nil
	}
	ps.SafetyStockLevel = level
	if err := s.repo.UpsertProductStock(ctx, tx, ps); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	_ = s.cache.DeleteStockSnapshot(ctx, sku)
	if ps.AvailableQuantity() < ps.SafetyStockLevel {
		payload, _ := json.Marshal(map[string]interface{}{"sku": ps.SKU, "warehouse_id": ps.WarehouseID, "available": ps.AvailableQuantity(), "threshold": ps.SafetyStockLevel})
		_ = s.publisher.Publish("inventory.stock.low", payload)
	}
	return nil
}
