package postgres

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/cmpnion/adp-final/services/inventory/internal/domain/entities"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

type PostgresRepo struct{ db *sql.DB }

func NewPostgresRepo(db *sql.DB) *PostgresRepo { return &PostgresRepo{db: db} }

func (r *PostgresRepo) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, &sql.TxOptions{})
}

func (r *PostgresRepo) GetProductStockForUpdate(ctx context.Context, tx *sql.Tx, sku string, warehouseID string) (*entities.ProductStock, error) {
	q := `SELECT id, sku, warehouse_id, total_qty, reserved_qty, safety_stock_level, created_at, updated_at FROM product_stocks WHERE sku=$1 AND warehouse_id=$2 FOR UPDATE`
	row := tx.QueryRowContext(ctx, q, sku, warehouseID)
	var s entities.ProductStock
	if err := row.Scan(&s.ID, &s.SKU, &s.WarehouseID, &s.TotalQuantity, &s.ReservedQuantity, &s.SafetyStockLevel, &s.CreatedAt, &s.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &s, nil
}

func (r *PostgresRepo) UpsertProductStock(ctx context.Context, tx *sql.Tx, s *entities.ProductStock) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
		s.CreatedAt = time.Now()
	}
	s.UpdatedAt = time.Now()
	q := `INSERT INTO product_stocks (id, sku, warehouse_id, total_qty, reserved_qty, safety_stock_level, created_at, updated_at)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	ON CONFLICT (sku, warehouse_id) DO UPDATE SET total_qty = $4, reserved_qty = $5, safety_stock_level = $6, updated_at = $8`
	_, err := tx.ExecContext(ctx, q, s.ID, s.SKU, s.WarehouseID, s.TotalQuantity, s.ReservedQuantity, s.SafetyStockLevel, s.CreatedAt, s.UpdatedAt)
	return err
}

func (r *PostgresRepo) CreateReservation(ctx context.Context, tx *sql.Tx, rr *entities.StockReservation) error {
	if rr.ID == "" {
		rr.ID = uuid.New().String()
		rr.CreatedAt = time.Now()
	}
	q := `INSERT INTO stock_reservations (id, order_id, sku, warehouse_id, quantity, status, expires_at, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`
	_, err := tx.ExecContext(ctx, q, rr.ID, rr.OrderID, rr.SKU, rr.WarehouseID, rr.Quantity, string(rr.Status), rr.ExpiresAt, rr.CreatedAt)
	return err
}

func (r *PostgresRepo) GetReservationByOrder(ctx context.Context, tx *sql.Tx, orderID string) ([]*entities.StockReservation, error) {
	q := `SELECT id, order_id, sku, warehouse_id, quantity, status, expires_at, created_at FROM stock_reservations WHERE order_id=$1`
	var (
		rows *sql.Rows
		err  error
	)
	if tx != nil {
		rows, err = tx.QueryContext(ctx, q, orderID)
	} else {
		rows, err = r.db.QueryContext(ctx, q, orderID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*entities.StockReservation{}
	for rows.Next() {
		var rr entities.StockReservation
		var status string
		if err := rows.Scan(&rr.ID, &rr.OrderID, &rr.SKU, &rr.WarehouseID, &rr.Quantity, &status, &rr.ExpiresAt, &rr.CreatedAt); err != nil {
			return nil, err
		}
		rr.Status = entities.ReservationStatus(status)
		out = append(out, &rr)
	}
	return out, nil
}

func (r *PostgresRepo) ListExpiredReservations(ctx context.Context, tx *sql.Tx, before time.Time) ([]*entities.StockReservation, error) {
	q := `SELECT id, order_id, sku, warehouse_id, quantity, status, expires_at, created_at
	      FROM stock_reservations
	      WHERE expires_at IS NOT NULL
	        AND expires_at <= $1
	        AND status = $2
	      ORDER BY order_id, id
	      FOR UPDATE SKIP LOCKED`
	var (
		rows *sql.Rows
		err  error
	)
	if tx != nil {
		rows, err = tx.QueryContext(ctx, q, before, string(entities.Reserved))
	} else {
		rows, err = r.db.QueryContext(ctx, q, before, string(entities.Reserved))
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*entities.StockReservation{}
	for rows.Next() {
		var rr entities.StockReservation
		var status string
		if err := rows.Scan(&rr.ID, &rr.OrderID, &rr.SKU, &rr.WarehouseID, &rr.Quantity, &status, &rr.ExpiresAt, &rr.CreatedAt); err != nil {
			return nil, err
		}
		rr.Status = entities.ReservationStatus(status)
		out = append(out, &rr)
	}
	return out, rows.Err()
}

func (r *PostgresRepo) DeleteReservation(ctx context.Context, tx *sql.Tx, reservationID string) error {
	q := `DELETE FROM stock_reservations WHERE id=$1`
	if tx != nil {
		_, err := tx.ExecContext(ctx, q, reservationID)
		return err
	}
	_, err := r.db.ExecContext(ctx, q, reservationID)
	return err
}

func (r *PostgresRepo) UpdateReservationStatus(ctx context.Context, tx *sql.Tx, reservationID string, status string) error {
	q := `UPDATE stock_reservations SET status=$1 WHERE id=$2`
	_, err := tx.ExecContext(ctx, q, status, reservationID)
	return err
}

func (r *PostgresRepo) UpdateReservationQuantity(ctx context.Context, tx *sql.Tx, reservationID string, newQty int64) error {
	q := `UPDATE stock_reservations SET quantity=$1 WHERE id=$2`
	_, err := tx.ExecContext(ctx, q, newQty, reservationID)
	return err
}

func (r *PostgresRepo) CreateMovement(ctx context.Context, tx *sql.Tx, m *entities.StockMovement) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
		m.CreatedAt = time.Now()
	}
	q := `INSERT INTO stock_movements (id, sku, warehouse_id, type, quantity, reference_id, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7)`
	_, err := tx.ExecContext(ctx, q, m.ID, m.SKU, m.WarehouseID, string(m.Type), m.Quantity, m.ReferenceID, m.CreatedAt)
	return err
}

func (r *PostgresRepo) GetLowStock(ctx context.Context, limit int) ([]*entities.ProductStock, error) {
	q := `SELECT id, sku, warehouse_id, total_qty, reserved_qty, safety_stock_level, created_at, updated_at
	      FROM product_stocks
	      WHERE sku IN (
	          SELECT sku
	          FROM product_stocks
	          GROUP BY sku
	          HAVING SUM(total_qty - reserved_qty) < 100
	      )
	      ORDER BY sku, warehouse_id
	      LIMIT $1`
	rows, err := r.db.QueryContext(ctx, q, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*entities.ProductStock{}
	for rows.Next() {
		var s entities.ProductStock
		if err := rows.Scan(&s.ID, &s.SKU, &s.WarehouseID, &s.TotalQuantity, &s.ReservedQuantity, &s.SafetyStockLevel, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &s)
	}
	return out, nil
}

func (r *PostgresRepo) ListStocksBySKU(ctx context.Context, sku string) ([]*entities.ProductStock, error) {
	q := `SELECT id, sku, warehouse_id, total_qty, reserved_qty, safety_stock_level, created_at, updated_at FROM product_stocks WHERE sku=$1`
	rows, err := r.db.QueryContext(ctx, q, sku)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*entities.ProductStock{}
	for rows.Next() {
		var s entities.ProductStock
		if err := rows.Scan(&s.ID, &s.SKU, &s.WarehouseID, &s.TotalQuantity, &s.ReservedQuantity, &s.SafetyStockLevel, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &s)
	}
	return out, nil
}

func (r *PostgresRepo) ListStocksByWarehouse(ctx context.Context, warehouseID string) ([]*entities.ProductStock, error) {
	q := `SELECT id, sku, warehouse_id, total_qty, reserved_qty, safety_stock_level, created_at, updated_at FROM product_stocks WHERE warehouse_id=$1`
	rows, err := r.db.QueryContext(ctx, q, warehouseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*entities.ProductStock{}
	for rows.Next() {
		var s entities.ProductStock
		if err := rows.Scan(&s.ID, &s.SKU, &s.WarehouseID, &s.TotalQuantity, &s.ReservedQuantity, &s.SafetyStockLevel, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &s)
	}
	return out, nil
}

func (r *PostgresRepo) CreateWarehouse(ctx context.Context, tx *sql.Tx, w *entities.Warehouse) error {
	if w.ID == "" {
		w.ID = uuid.New().String()
		w.CreatedAt = time.Now()
	}
	q := `INSERT INTO warehouses (id, name, location, is_active, created_at) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, location=EXCLUDED.location, is_active=EXCLUDED.is_active`
	_, err := tx.ExecContext(ctx, q, w.ID, w.Name, w.Location, w.IsActive, w.CreatedAt)
	return err
}

func (r *PostgresRepo) UpdateWarehouse(ctx context.Context, tx *sql.Tx, w *entities.Warehouse) error {
	q := `UPDATE warehouses SET name=$1, location=$2, is_active=$3 WHERE id=$4`
	_, err := tx.ExecContext(ctx, q, w.Name, w.Location, w.IsActive, w.ID)
	return err
}

func (r *PostgresRepo) ListWarehouses(ctx context.Context) ([]*entities.Warehouse, error) {
	q := `SELECT id, name, location, is_active, created_at FROM warehouses`
	rows, err := r.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*entities.Warehouse{}
	for rows.Next() {
		var w entities.Warehouse
		if err := rows.Scan(&w.ID, &w.Name, &w.Location, &w.IsActive, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &w)
	}
	return out, nil
}

func (r *PostgresRepo) LockStocksInDB(ctx context.Context, tx *sql.Tx, keys []string) error {
	// keys are sku:warehouse
	for _, k := range keys {
		sku, wh, ok := strings.Cut(k, ":")
		if !ok || sku == "" || wh == "" {
			continue
		}
		q := `SELECT id FROM product_stocks WHERE sku=$1 AND warehouse_id=$2 FOR UPDATE`
		if _, err := tx.ExecContext(ctx, q, sku, wh); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepo) FindWarehouseForReservation(ctx context.Context, tx *sql.Tx, sku string, qty int64) (string, error) {
	// NOTE: No FOR UPDATE lock here - Redis lock already protects this
	// FOR UPDATE causes DB-level lock contention, slowing down the query significantly
	q := `SELECT warehouse_id FROM product_stocks WHERE sku=$1 AND (total_qty - reserved_qty) >= $2 ORDER BY (total_qty - reserved_qty) DESC LIMIT 1`
	row := tx.QueryRowContext(ctx, q, sku, qty)
	var wid string
	if err := row.Scan(&wid); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return wid, nil
}
