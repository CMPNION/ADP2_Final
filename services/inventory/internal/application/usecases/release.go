package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cmpnion/adp-final/services/inventory/internal/domain"
	"github.com/cmpnion/adp-final/services/inventory/internal/domain/entities"
)

// ReleaseService cancels all reservations for an order
type ReleaseService struct {
	repo      domain.Repository
	locker    Locker
	cache     Cache
	publisher Publisher
}

func NewReleaseService(r domain.Repository, l Locker, c Cache, p Publisher) *ReleaseService {
	return &ReleaseService{repo: r, locker: l, cache: c, publisher: p}
}

// ReleaseStock releases (cancels) all Reserved reservations for an order
func (s *ReleaseService) ReleaseStock(ctx context.Context, orderID string) (int64, error) {
	if orderID == "" {
		return 0, fmt.Errorf("order_id is required")
	}

	lockKey := fmt.Sprintf("lock:order:%s", orderID)
	ok, err := s.locker.Lock(ctx, lockKey, 8*time.Second)
	if err != nil || !ok {
		return 0, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer s.locker.Unlock(ctx, lockKey)

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// Get all reservations for this order
	reservations, err := s.repo.GetReservationByOrder(ctx, tx, orderID)
	if err != nil {
		return 0, err
	}

	releasedCount := int64(0)
	skus := make(map[string]struct{})

	// Release only Reserved status reservations
	for _, res := range reservations {
		// Skip already Confirmed or Released
		if res.Status != entities.Reserved {
			continue
		}

		// Fetch stock
		ps, err := s.repo.GetProductStockForUpdate(ctx, tx, res.SKU, res.WarehouseID)
		if err != nil {
			return 0, err
		}
		if ps == nil {
			return 0, fmt.Errorf("stock not found: %s@%s", res.SKU, res.WarehouseID)
		}

		// Release reserved qty
		ps.Release(res.Quantity)
		if err := s.repo.UpsertProductStock(ctx, tx, ps); err != nil {
			return 0, err
		}

		// Log movement
		mov := &entities.StockMovement{
			ID:          fmt.Sprintf("mov-%d", time.Now().UnixNano()),
			SKU:         res.SKU,
			WarehouseID: res.WarehouseID,
			Type:        entities.Release,
			Quantity:    res.Quantity,
			ReferenceID: orderID,
			CreatedAt:   time.Now(),
		}
		if err := s.repo.CreateMovement(ctx, tx, mov); err != nil {
			return 0, err
		}

		// Mark reservation as Released
		if err := s.repo.UpdateReservationStatus(ctx, tx, res.ID, string(entities.Released)); err != nil {
			return 0, err
		}

		releasedCount += res.Quantity
		skus[res.SKU] = struct{}{}

		// Publish event
		if payload, err := json.Marshal(map[string]interface{}{
			"order_id":      orderID,
			"reservation_id": res.ID,
			"sku":           res.SKU,
			"warehouse_id":  res.WarehouseID,
			"quantity":      res.Quantity,
		}); err == nil && s.publisher != nil {
			_ = s.publisher.Publish("inventory.stock.released", payload)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	// Cache bust
	for sku := range skus {
		_ = s.cache.DeleteStockSnapshot(ctx, sku)
	}

	return releasedCount, nil
}

