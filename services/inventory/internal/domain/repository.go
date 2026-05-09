package domain

import (
	"context"
	"database/sql"

	"github.com/yourorg/inventory/internal/domain/entities"
)

// Repository defines persistence operations required by usecases
// Methods that modify state accept an explicit *sql.Tx so callers can
// compose ACID transactions.
type Repository interface {
	BeginTx(ctx context.Context) (*sql.Tx, error)

	GetProductStockForUpdate(ctx context.Context, tx *sql.Tx, sku string, warehouseID string) (*entities.ProductStock, error)
	UpsertProductStock(ctx context.Context, tx *sql.Tx, s *entities.ProductStock) error

	CreateReservation(ctx context.Context, tx *sql.Tx, r *entities.StockReservation) error
	GetReservationByOrder(ctx context.Context, tx *sql.Tx, orderID string) ([]*entities.StockReservation, error)
	UpdateReservationStatus(ctx context.Context, tx *sql.Tx, reservationID string, status string) error

	CreateMovement(ctx context.Context, tx *sql.Tx, m *entities.StockMovement) error

	GetLowStock(ctx context.Context, limit int) ([]*entities.ProductStock, error)

	CreateWarehouse(ctx context.Context, tx *sql.Tx, w *entities.Warehouse) error
	UpdateWarehouse(ctx context.Context, tx *sql.Tx, w *entities.Warehouse) error
	ListWarehouses(ctx context.Context) ([]*entities.Warehouse, error)

	// Fallback DB-level locking: select for update across sku rows
	LockStocksInDB(ctx context.Context, tx *sql.Tx, keys []string) error
}
