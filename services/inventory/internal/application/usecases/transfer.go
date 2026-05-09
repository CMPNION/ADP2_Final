package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/yourorg/inventory/internal/domain"
	"github.com/yourorg/inventory/internal/domain/entities"
)

// Transfer stock between warehouses
func (s *ReserveService) TransferStock(ctx context.Context, sku string, from string, to string, qty int64, referenceID string) error {
	// Lock both source and destination
	keys := []string{fmt.Sprintf("lock:stock:%s:%s", sku, from), fmt.Sprintf("lock:stock:%s:%s", sku, to)}
	acquired := []string{}
	for _, k := range keys {
		ok, err := s.locker.Lock(ctx, k, 8*time.Second)
		if err != nil || !ok {
			for _, a := range acquired { _ = s.locker.Unlock(ctx, a) }
			return fmt.Errorf("failed to acquire lock: %w", err)
		}
		acquired = append(acquired, k)
	}
	defer func(){ for _, a := range acquired { _ = s.locker.Unlock(ctx, a) } }()

	tx, err := s.repo.BeginTx(ctx)
	if err != nil { return err }
	defer tx.Rollback()

	fromStock, err := s.repo.GetProductStockForUpdate(ctx, tx, sku, from)
	if err != nil { return err }
	if fromStock == nil { return fmt.Errorf("from stock not found") }
	if fromStock.AvailableQuantity() < qty { return fmt.Errorf("insufficient stock in source") }

	toStock, err := s.repo.GetProductStockForUpdate(ctx, tx, sku, to)
	if err != nil { return err }
	if toStock == nil {
		// create empty stock
		toStock = &entities.ProductStock{ID: uuid.New().String(), SKU: sku, WarehouseID: to, TotalQuantity: 0, ReservedQuantity: 0, SafetyStockLevel: 0, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	}

	// move quantities
	fromStock.TotalQuantity -= qty
	if fromStock.TotalQuantity < 0 { fromStock.TotalQuantity = 0 }
	toStock.TotalQuantity += qty

	if err := s.repo.UpsertProductStock(ctx, tx, fromStock); err != nil { return err }
	if err := s.repo.UpsertProductStock(ctx, tx, toStock); err != nil { return err }

	// ledger entries
	outMov := &entities.StockMovement{ID: uuid.New().String(), SKU: sku, WarehouseID: from, Type: entities.Transfer, Quantity: -qty, ReferenceID: referenceID, CreatedAt: time.Now()}
	inMov := &entities.StockMovement{ID: uuid.New().String(), SKU: sku, WarehouseID: to, Type: entities.Transfer, Quantity: qty, ReferenceID: referenceID, CreatedAt: time.Now()}
	if err := s.repo.CreateMovement(ctx, tx, outMov); err != nil { return err }
	if err := s.repo.CreateMovement(ctx, tx, inMov); err != nil { return err }

	if err := tx.Commit(); err != nil { return err }

	// refresh cache and publish
	_ = s.cache.DeleteStockSnapshot(ctx, sku)
	evt := map[string]interface{}{"sku": sku, "from": from, "to": to, "qty": qty, "reference": referenceID}
	if payload, e := json.Marshal(evt); e == nil { _ = s.publisher.Publish("inventory.stock.transferred", payload) }

	return nil
}
