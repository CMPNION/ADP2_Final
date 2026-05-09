package usecases

import (
	"context"
	"encoding/json"
	"fmt"
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
	results := make([]ReservationResult, 0, len(items))
	for _, item := range items {
		lockKey := fmt.Sprintf("lock:stock:%s:%s", item.SKU, item.WarehouseID)
		ok, err := s.locker.Lock(ctx, lockKey, 8*time.Second)
		if err != nil || !ok {
			return nil, fmt.Errorf("failed to lock stock: %w", err)
		}

		tx, err := s.repo.BeginTx(ctx)
		if err != nil {
			_ = s.locker.Unlock(ctx, lockKey)
			return nil, err
		}

		ps, err := s.repo.GetProductStockForUpdate(ctx, tx, item.SKU, item.WarehouseID)
		if err != nil {
			_ = tx.Rollback()
			_ = s.locker.Unlock(ctx, lockKey)
			return nil, err
		}
		if ps == nil || ps.AvailableQuantity() < item.Quantity {
			_ = tx.Rollback()
			_ = s.locker.Unlock(ctx, lockKey)
			return nil, fmt.Errorf("insufficient stock")
		}

		ps.ReservedQuantity += item.Quantity
		if err := s.repo.UpsertProductStock(ctx, tx, ps); err != nil {
			_ = tx.Rollback()
			_ = s.locker.Unlock(ctx, lockKey)
			return nil, err
		}

		reservation := &entities.StockReservation{
			ID:          uuid.New().String(),
			OrderID:     orderID,
			SKU:         item.SKU,
			WarehouseID: item.WarehouseID,
			Quantity:    item.Quantity,
			Status:      entities.Reserved,
			ExpiresAt:   time.Now().Add(30 * time.Minute),
			CreatedAt:   time.Now(),
		}
		if err := s.repo.CreateReservation(ctx, tx, reservation); err != nil {
			_ = tx.Rollback()
			_ = s.locker.Unlock(ctx, lockKey)
			return nil, err
		}

		movement := &entities.StockMovement{
			ID:          uuid.New().String(),
			SKU:         item.SKU,
			WarehouseID: item.WarehouseID,
			Type:        entities.Reserve,
			Quantity:    item.Quantity,
			ReferenceID: orderID,
			CreatedAt:   time.Now(),
		}
		if err := s.repo.CreateMovement(ctx, tx, movement); err != nil {
			_ = tx.Rollback()
			_ = s.locker.Unlock(ctx, lockKey)
			return nil, err
		}

		if err := tx.Commit(); err != nil {
			_ = s.locker.Unlock(ctx, lockKey)
			return nil, err
		}

		_ = s.cache.DeleteStockSnapshot(ctx, item.SKU)
		if payload, err := json.Marshal(map[string]any{
			"order_id": orderID,
			"sku":      item.SKU,
			"quantity": item.Quantity,
		}); err == nil && s.publisher != nil {
			_ = s.publisher.Publish("inventory.stock.reserved", payload)
		}
		_ = s.locker.Unlock(ctx, lockKey)
		results = append(results, ReservationResult{
			ReservationID: reservation.ID,
			SKU:           item.SKU,
			WarehouseID:   item.WarehouseID,
			Quantity:      item.Quantity,
		})
	}
	return results, nil
}
