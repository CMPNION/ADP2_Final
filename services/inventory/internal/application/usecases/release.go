package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmpnion/adp-final/services/inventory/internal/domain"
	"github.com/cmpnion/adp-final/services/inventory/internal/domain/entities"
)

// ReleaseService releases reservations for an order
type ReleaseService struct {
	repo      domain.Repository
	locker    Locker
	cache     Cache
	publisher Publisher
}

func NewReleaseService(r domain.Repository, l Locker, c Cache, p Publisher) *ReleaseService {
	return &ReleaseService{repo: r, locker: l, cache: c, publisher: p}
}

func (s *ReleaseService) ReleaseStock(ctx context.Context, orderID string) error {
	// Acquire generic lock for order to avoid concurrent confirm/release
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
		if r.Status == entities.Confirmed {
			return fmt.Errorf("order %s is already confirmed", orderID)
		}
	}

	for _, r := range reservations {
		// Only process Reserved status - skip Already Released/Confirmed
		if r.Status != entities.Reserved {
			continue
		}
		// fetch product stock
		ps, err := s.repo.GetProductStockForUpdate(ctx, tx, r.SKU, r.WarehouseID)
		if err != nil {
			return err
		}
		if ps == nil {
			return fmt.Errorf("product stock not found %s@%s", r.SKU, r.WarehouseID)
		}

		// Only release if there's actually reserved quantity to release
		// This prevents double-release if function called multiple times
		if ps.ReservedQuantity < r.Quantity {
			return fmt.Errorf("inconsistent state: reservation qty %d exceeds reserved %d for %s@%s", r.Quantity, ps.ReservedQuantity, r.SKU, r.WarehouseID)
		}

		// release reserved quantity back to available
		ps.Release(r.Quantity)
		if err := s.repo.UpsertProductStock(ctx, tx, ps); err != nil {
			return err
		}
		// ledger
		mov := &entities.StockMovement{SKU: r.SKU, WarehouseID: r.WarehouseID, Type: entities.Release, Quantity: r.Quantity, ReferenceID: orderID, CreatedAt: time.Now()}
		if err := s.repo.CreateMovement(ctx, tx, mov); err != nil {
			return err
		}
		// update reservation status to Released
		if err := s.repo.UpdateReservationStatus(ctx, tx, r.ID, string(entities.Released)); err != nil {
			return err
		}
		// publish event
		evt := map[string]interface{}{"order_id": orderID, "sku": r.SKU, "quantity": r.Quantity, "warehouse_id": r.WarehouseID}
		if payload, e := json.Marshal(evt); e == nil && s.publisher != nil {
			_ = s.publisher.Publish("inventory.stock.released", payload)
		}
		// refresh cache
		_ = s.cache.DeleteStockSnapshot(ctx, r.SKU)
	}

	return tx.Commit()
}
