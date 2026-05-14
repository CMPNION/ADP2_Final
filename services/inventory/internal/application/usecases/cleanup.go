package usecases

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/cmpnion/adp-final/services/inventory/internal/domain/entities"
	"github.com/google/uuid"
)

func (s *ReserveService) CleanupExpiredReservations(ctx context.Context) (int, error) {
	now := time.Now()
	expired, err := s.repo.ListExpiredReservations(ctx, nil, now)
	if err != nil {
		return 0, err
	}
	if len(expired) == 0 {
		return 0, nil
	}

	orderIDs := make([]string, 0, len(expired))
	seen := make(map[string]struct{}, len(expired))
	for _, r := range expired {
		if _, ok := seen[r.OrderID]; ok {
			continue
		}
		seen[r.OrderID] = struct{}{}
		orderIDs = append(orderIDs, r.OrderID)
	}
	sort.Strings(orderIDs)

	acquired := make([]string, 0, len(orderIDs))
	for _, orderID := range orderIDs {
		if s.locker == nil {
			break
		}
		lockKey := fmt.Sprintf("lock:order:%s", orderID)
		ok, lockErr := s.locker.Lock(ctx, lockKey, 8*time.Second)
		if lockErr != nil || !ok {
			for _, locked := range acquired {
				_ = s.locker.Unlock(ctx, locked)
			}
			if lockErr != nil {
				return 0, fmt.Errorf("failed to lock order for cleanup: %w", lockErr)
			}
			return 0, fmt.Errorf("failed to lock order for cleanup")
		}
		acquired = append(acquired, lockKey)
	}
	defer func() {
		if s.locker == nil {
			return
		}
		for _, locked := range acquired {
			_ = s.locker.Unlock(ctx, locked)
		}
	}()

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	processed := 0
	for _, orderID := range orderIDs {
		reservations, err := s.repo.GetReservationByOrder(ctx, tx, orderID)
		if err != nil {
			return 0, err
		}
		for _, reservation := range reservations {
			if reservation.Status != entities.Reserved || reservation.ExpiresAt.After(now) || reservation.ExpiresAt.IsZero() {
				continue
			}
			ps, err := s.repo.GetProductStockForUpdate(ctx, tx, reservation.SKU, reservation.WarehouseID)
			if err != nil {
				return 0, err
			}
			if ps == nil {
				return 0, fmt.Errorf("product stock not found %s@%s", reservation.SKU, reservation.WarehouseID)
			}
			ps.Release(reservation.Quantity)
			if err := s.repo.UpsertProductStock(ctx, tx, ps); err != nil {
				return 0, err
			}
			mov := &entities.StockMovement{
				ID:          uuid.New().String(),
				SKU:         reservation.SKU,
				WarehouseID: reservation.WarehouseID,
				Type:        entities.Release,
				Quantity:    reservation.Quantity,
				ReferenceID: reservation.OrderID,
				CreatedAt:   now,
			}
			if err := s.repo.CreateMovement(ctx, tx, mov); err != nil {
				return 0, err
			}
			if err := s.repo.DeleteReservation(ctx, tx, reservation.ID); err != nil {
				return 0, err
			}
			_ = s.cache.DeleteStockSnapshot(ctx, reservation.SKU)
			processed++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return processed, nil
}
