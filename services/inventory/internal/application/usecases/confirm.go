package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"omnichannel/inventory/internal/domain"
	"omnichannel/inventory/internal/domain/entities"
)

// ConfirmService finalizes reservations into deductions
type ConfirmService struct {
	repo      domain.Repository
	locker    Locker
	cache     Cache
	publisher Publisher
}

func NewConfirmService(r domain.Repository, l Locker, c Cache, p Publisher) *ConfirmService {
	return &ConfirmService{repo: r, locker: l, cache: c, publisher: p}
}

func (s *ConfirmService) ConfirmStockDeduction(ctx context.Context, orderID string) error {
	lockKey := fmt.Sprintf("lock:order:%s", orderID)
	ok, err := s.locker.Lock(ctx, lockKey, 8*time.Second)
	if err != nil || !ok {
		return fmt.Errorf("failed to lock order: %w", err)
	}
	defer s.locker.Unlock(ctx, lockKey)

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	reservations, err := s.repo.GetReservationByOrder(ctx, tx, orderID)
	if err != nil {
		return err
	}
	if len(reservations) == 0 {
		return nil
	}

	for _, r := range reservations {
		if r.Status != entities.Reserved {
			continue
		}
		ps, err := s.repo.GetProductStockForUpdate(ctx, tx, r.SKU, r.WarehouseID)
		if err != nil {
			return err
		}
		if ps == nil {
			return fmt.Errorf("product stock not found %s@%s", r.SKU, r.WarehouseID)
		}
		// deduct
		ps.Deduct(r.Quantity)
		if err := s.repo.UpsertProductStock(ctx, tx, ps); err != nil {
			return err
		}
		// ledger OUT
		mov := &entities.StockMovement{SKU: r.SKU, WarehouseID: r.WarehouseID, Type: entities.Out, Quantity: r.Quantity, ReferenceID: orderID, CreatedAt: time.Now()}
		if err := s.repo.CreateMovement(ctx, tx, mov); err != nil {
			return err
		}
		if err := s.repo.UpdateReservationStatus(ctx, tx, r.ID, string(entities.Confirmed)); err != nil {
			return err
		}
		// publish confirmed
		evt := map[string]interface{}{"order_id": orderID, "sku": r.SKU}
		if payload, e := json.Marshal(evt); e == nil && s.publisher != nil {
			_ = s.publisher.Publish("inventory.stock.confirmed", payload)
		}
		// refresh cache
		_ = s.cache.DeleteStockSnapshot(ctx, r.SKU)
		// safety check
		if ps.AvailableQuantity() < ps.SafetyStockLevel {
			payload, _ := json.Marshal(map[string]interface{}{"sku": ps.SKU, "warehouse_id": ps.WarehouseID, "available": ps.AvailableQuantity(), "threshold": ps.SafetyStockLevel})
			if s.publisher != nil {
				_ = s.publisher.Publish("inventory.stock.low", payload)
			}
		}
	}

	return tx.Commit()
}
