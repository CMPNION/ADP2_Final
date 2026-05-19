package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/cmpnion/adp-final/services/inventory/internal/domain"
	"github.com/cmpnion/adp-final/services/inventory/internal/domain/entities"
	"github.com/google/uuid"
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
// ReserveStock - simple 1:1 reservation
// orderID + sku + qty + warehouse = 1 reservation record
// No merging, no auto-warehouse selection
// Returns the created reservation
func (s *ReserveService) ReserveStock(ctx context.Context, orderID string, sku string, warehouseID string, qty int64) (*ReservationResult, error) {
	// Validation
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, fmt.Errorf("order_id is required")
	}
	sku = strings.TrimSpace(sku)
	if sku == "" {
		return nil, fmt.Errorf("sku is required")
	}
	if qty <= 0 {
		return nil, fmt.Errorf("qty must be > 0")
	}
	warehouseID = strings.TrimSpace(warehouseID)

	// Lock this specific SKU (before we know warehouse)
	// Use wildcard lock if warehouse not specified yet
	lockKey := fmt.Sprintf("lock:stock:%s:*", sku)
	if warehouseID != "" {
		lockKey = fmt.Sprintf("lock:stock:%s:%s", sku, warehouseID)
	}
	ok, err := s.locker.Lock(ctx, lockKey, 8*time.Second)
	if err != nil || !ok {
		return nil, fmt.Errorf("failed to acquire lock: %w", err)
	}
	defer s.locker.Unlock(ctx, lockKey)

	// Single transaction for everything
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// Find warehouse if not specified
	if warehouseID == "" {
		found, err := s.repo.FindWarehouseForReservation(ctx, tx, sku, qty)
		if err != nil {
			return nil, err
		}
		if found == "" {
			return nil, fmt.Errorf("no warehouse has %d units of %s", qty, sku)
		}
		warehouseID = found
		
		// Update lock to specific warehouse+sku
		if err := s.locker.Unlock(ctx, lockKey); err != nil {
			return nil, fmt.Errorf("failed to unlock: %w", err)
		}
		lockKey = fmt.Sprintf("lock:stock:%s:%s", sku, warehouseID)
		ok, err := s.locker.Lock(ctx, lockKey, 8*time.Second)
		if err != nil || !ok {
			return nil, fmt.Errorf("failed to acquire specific lock: %w", err)
		}
		defer s.locker.Unlock(ctx, lockKey)
	}

	// Check if this exact reservation already exists (idempotent)
	existing, err := s.repo.GetReservationByOrder(ctx, tx, orderID)
	if err != nil {
		return nil, err
	}
	for _, res := range existing {
		if res.SKU == sku && res.WarehouseID == warehouseID && res.Quantity == qty && res.Status == entities.Reserved {
			// Already exists, return it
			return &ReservationResult{
				ReservationID: res.ID,
				SKU:           res.SKU,
				WarehouseID:   res.WarehouseID,
				Quantity:      res.Quantity,
			}, nil
		}
	}

	// Get stock for update
	ps, err := s.repo.GetProductStockForUpdate(ctx, tx, sku, warehouseID)
	if err != nil {
		return nil, err
	}
	if ps == nil {
		return nil, fmt.Errorf("stock not found: %s@%s", sku, warehouseID)
	}

	// Check availability
	if ps.AvailableQuantity() < qty {
		return nil, fmt.Errorf("insufficient stock: need %d, have %d", qty, ps.AvailableQuantity())
	}

	// Reserve
	ps.Reserve(qty)
	if err := s.repo.UpsertProductStock(ctx, tx, ps); err != nil {
		return nil, err
	}

	// Create reservation record
	reservation := &entities.StockReservation{
		ID:          uuid.New().String(),
		OrderID:     orderID,
		SKU:         sku,
		WarehouseID: warehouseID,
		Quantity:    qty,
		Status:      entities.Reserved,
		ExpiresAt:   time.Now().Add(30 * time.Minute),
		CreatedAt:   time.Now(),
	}
	if err := s.repo.CreateReservation(ctx, tx, reservation); err != nil {
		return nil, err
	}

	// Log movement
	mov := &entities.StockMovement{
		ID:          uuid.New().String(),
		SKU:         sku,
		WarehouseID: warehouseID,
		Type:        entities.Reserve,
		Quantity:    qty,
		ReferenceID: orderID,
		CreatedAt:   time.Now(),
	}
	if err := s.repo.CreateMovement(ctx, tx, mov); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Cache bust
	_ = s.cache.DeleteStockSnapshot(ctx, sku)

	// Publish event
	if payload, err := json.Marshal(map[string]interface{}{
		"order_id":      orderID,
		"sku":           sku,
		"warehouse_id":  warehouseID,
		"quantity":      qty,
		"reservation_id": reservation.ID,
	}); err == nil && s.publisher != nil {
		_ = s.publisher.Publish("inventory.stock.reserved", payload)
	}

	return &ReservationResult{
		ReservationID: reservation.ID,
		SKU:           sku,
		WarehouseID:   warehouseID,
		Quantity:      qty,
	}, nil
}

