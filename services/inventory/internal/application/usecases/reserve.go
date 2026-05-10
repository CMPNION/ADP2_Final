package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"omnichannel/inventory/internal/domain"
	"omnichannel/inventory/internal/domain/entities"
)

type Locker interface {
	Lock(ctx context.Context, key string, ttl time.Duration) (bool, error)
	Unlock(ctx context.Context, key string) error
}

type Cache interface {
	DeleteStockSnapshot(ctx context.Context, sku string) error
}

type Publisher interface {
	Publish(subject string, payload []byte) error
}

type ItemReq struct {
	SKU         string
	WarehouseID string
	Quantity    int64
}

type ReservationResult struct {
	ReservationID string
	SKU           string
	WarehouseID   string
	Quantity      int64
}

type ReserveService struct {
	repo      domain.Repository
	locker    Locker
	cache     Cache
	publisher Publisher
}

func NewReserveService(r domain.Repository, l Locker, c Cache, p Publisher) *ReserveService {
	return &ReserveService{repo: r, locker: l, cache: c, publisher: p}
}

func (s *ReserveService) ReserveStock(ctx context.Context, orderID string, items []ItemReq) ([]ReservationResult, error) {
	if len(items) == 0 {
		return nil, nil
	}

	type preparedItem struct {
		ItemReq
	}

	prepared := make([]preparedItem, 0, len(items))
	lockKeySet := make(map[string]struct{}, len(items))
	for _, item := range items {
		lockKey := fmt.Sprintf("lock:stock:%s:%s", item.SKU, item.WarehouseID)
		if item.WarehouseID == "" {
			lockKey = fmt.Sprintf("lock:stock:%s:*", item.SKU)
		}
		prepared = append(prepared, preparedItem{ItemReq: item})
		lockKeySet[lockKey] = struct{}{}
	}
	lockKeys := make([]string, 0, len(lockKeySet))
	for key := range lockKeySet {
		lockKeys = append(lockKeys, key)
	}
	sort.Strings(lockKeys)

	acquired := make([]string, 0, len(lockKeys))
	for _, key := range lockKeys {
		ok, err := s.locker.Lock(ctx, key, 8*time.Second)
		if err != nil || !ok {
			for _, locked := range acquired {
				_ = s.locker.Unlock(ctx, locked)
			}
			return nil, fmt.Errorf("failed to lock stock: %w", err)
		}
		acquired = append(acquired, key)
	}
	defer func() {
		for _, key := range acquired {
			_ = s.locker.Unlock(ctx, key)
		}
	}()

	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	existing, err := s.repo.GetReservationByOrder(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}

	existingBySKU := make(map[string]*entities.StockReservation, len(existing))
	for _, r := range existing {
		existingBySKU[r.SKU] = r
	}

	results := make([]ReservationResult, 0, len(prepared))
	now := time.Now()
	for _, item := range prepared {
		if prev, ok := existingBySKU[item.SKU]; ok {
			if prev.Quantity != item.Quantity {
				return nil, fmt.Errorf("reservation already exists for sku %s with different quantity", item.SKU)
			}
			results = append(results, ReservationResult{
				ReservationID: prev.ID,
				SKU:           prev.SKU,
				WarehouseID:   prev.WarehouseID,
				Quantity:      prev.Quantity,
			})
			continue
		}

		warehouseID := item.WarehouseID
		if warehouseID == "" {
			warehouseID, err = s.repo.FindWarehouseForReservation(ctx, tx, item.SKU, item.Quantity)
			if err != nil {
				return nil, err
			}
			if warehouseID == "" {
				return nil, fmt.Errorf("no warehouse has available stock for sku %s", item.SKU)
			}
		}

		ps, err := s.repo.GetProductStockForUpdate(ctx, tx, item.SKU, warehouseID)
		if err != nil {
			return nil, err
		}
		if ps == nil {
			return nil, fmt.Errorf("stock not found for sku %s warehouse %s", item.SKU, warehouseID)
		}
		if ps.AvailableQuantity() < item.Quantity {
			return nil, fmt.Errorf("insufficient stock for sku %s warehouse %s", item.SKU, warehouseID)
		}

		ps.Reserve(item.Quantity)
		if err := s.repo.UpsertProductStock(ctx, tx, ps); err != nil {
			return nil, err
		}

		reservation := &entities.StockReservation{
			ID:          uuid.New().String(),
			OrderID:     orderID,
			SKU:         item.SKU,
			WarehouseID: warehouseID,
			Quantity:    item.Quantity,
			Status:      entities.Reserved,
			ExpiresAt:   now.Add(30 * time.Minute),
			CreatedAt:   now,
		}
		if err := s.repo.CreateReservation(ctx, tx, reservation); err != nil {
			return nil, err
		}

		movement := &entities.StockMovement{
			ID:          uuid.New().String(),
			SKU:         item.SKU,
			WarehouseID: warehouseID,
			Type:        entities.Reserve,
			Quantity:    item.Quantity,
			ReferenceID: orderID,
			CreatedAt:   now,
		}
		if err := s.repo.CreateMovement(ctx, tx, movement); err != nil {
			return nil, err
		}

		results = append(results, ReservationResult{
			ReservationID: reservation.ID,
			SKU:           reservation.SKU,
			WarehouseID:   reservation.WarehouseID,
			Quantity:      reservation.Quantity,
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	for _, item := range prepared {
		_ = s.cache.DeleteStockSnapshot(ctx, item.SKU)
	}
	if payload, err := json.Marshal(map[string]any{
		"order_id": orderID,
		"items":    results,
	}); err == nil && s.publisher != nil {
		_ = s.publisher.Publish("inventory.stock.reserved", payload)
	}

	return results, nil
}
