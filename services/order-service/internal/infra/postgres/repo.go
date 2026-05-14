package postgres

import (
	"context"
	"database/sql"
	"strings"

	"github.com/cmpnion/adp-final/services/order/internal/domain"
	"github.com/lib/pq"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

// AdjustOrderItemQuantity adds delta to order_items.quantity for (order_id, sku).
// If row doesn't exist and delta>0 it inserts a new row. If resulting quantity <=0 the row is deleted.
func (r *Repo) AdjustOrderItemQuantity(ctx context.Context, orderID, sku string, delta int64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Try to update existing row
	res, err := tx.ExecContext(ctx, `UPDATE order_items SET quantity = quantity + $3 WHERE order_id = $1 AND sku = $2`, orderID, sku, delta)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 && delta > 0 {
		// insert new row
		if _, err := tx.ExecContext(ctx, `INSERT INTO order_items (order_id, sku, quantity) VALUES ($1, $2, $3)`, orderID, sku, delta); err != nil {
			return err
		}
	}

	// Ensure no non-positive quantities remain
	// Delete any rows with quantity <= 0
	if _, err := tx.ExecContext(ctx, `DELETE FROM order_items WHERE order_id = $1 AND sku = $2 AND quantity <= 0`, orderID, sku); err != nil {
		return err
	}

	return tx.Commit()
}
func (r *Repo) CreateOrder(ctx context.Context, o *domain.Order) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO orders (id, user_id, status)
		VALUES ($1, $2, $3)
	`, o.ID, o.UserID, strings.ToUpper(strings.TrimSpace(o.Status))); err != nil {
		return err
	}

	for _, it := range o.Items {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO order_items (order_id, sku, quantity)
			VALUES ($1, $2, $3)
		`, o.ID, it.SKU, it.Quantity); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (r *Repo) GetOrder(ctx context.Context, orderID string) (*domain.Order, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, user_id, status FROM orders WHERE id = $1`, orderID)
	var o domain.Order
	if err := row.Scan(&o.ID, &o.UserID, &o.Status); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	items, err := r.orderItems(ctx, []string{orderID})
	if err != nil {
		return nil, err
	}
	o.Items = items[orderID]
	return &o, nil
}

func (r *Repo) UpdateStatus(ctx context.Context, orderID, status string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE orders SET status = $2, updated_at = NOW() WHERE id = $1`, orderID, status)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (r *Repo) BulkGetOrders(ctx context.Context, orderIDs []string) ([]domain.Order, error) {
	if len(orderIDs) == 0 {
		return []domain.Order{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, status
		FROM orders
		WHERE id = ANY($1)
	`, pq.Array(orderIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanOrders(ctx, rows)
}

func (r *Repo) ListOrdersByUser(ctx context.Context, userID string) ([]domain.Order, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, status
		FROM orders
		WHERE user_id = $1
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanOrders(ctx, rows)
}

func (r *Repo) ListOrdersByStatus(ctx context.Context, status string) ([]domain.Order, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if strings.TrimSpace(status) == "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, user_id, status
			FROM orders
			ORDER BY created_at DESC
		`)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT id, user_id, status
			FROM orders
			WHERE status = $1
			ORDER BY created_at DESC
		`, status)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return r.scanOrders(ctx, rows)
}

func (r *Repo) Stats(ctx context.Context) (int32, int32, int32, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*)::int4,
			COUNT(*) FILTER (WHERE status IN ('CREATED', 'PENDING'))::int4,
			COUNT(*) FILTER (WHERE status IN ('SHIPPED', 'COMPLETED'))::int4
		FROM orders
	`)
	var total, pending, completed int32
	if err := row.Scan(&total, &pending, &completed); err != nil {
		return 0, 0, 0, err
	}
	return total, pending, completed, nil
}

func (r *Repo) scanOrders(ctx context.Context, rows *sql.Rows) ([]domain.Order, error) {
	out := make([]domain.Order, 0)
	ids := make([]string, 0)
	for rows.Next() {
		var o domain.Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status); err != nil {
			return nil, err
		}
		out = append(out, o)
		ids = append(ids, o.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	itemsByOrder, err := r.orderItems(ctx, ids)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Items = itemsByOrder[out[i].ID]
	}
	return out, nil
}

func (r *Repo) orderItems(ctx context.Context, orderIDs []string) (map[string][]domain.OrderItem, error) {
	result := make(map[string][]domain.OrderItem, len(orderIDs))
	if len(orderIDs) == 0 {
		return result, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT order_id, sku, quantity
		FROM order_items
		WHERE order_id = ANY($1)
		ORDER BY id
	`, pq.Array(orderIDs))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var orderID string
		var item domain.OrderItem
		if err := rows.Scan(&orderID, &item.SKU, &item.Quantity); err != nil {
			return nil, err
		}
		result[orderID] = append(result[orderID], item)
	}
	return result, rows.Err()
}
