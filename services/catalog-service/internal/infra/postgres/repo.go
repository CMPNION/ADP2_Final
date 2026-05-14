package postgres

import (
	"context"
	"database/sql"
	"strings"

	"github.com/cmpnion/adp-final/services/catalog/internal/application/usecases"
	"github.com/cmpnion/adp-final/services/catalog/internal/domain"
	"github.com/lib/pq"
)

type Repo struct {
	db *sql.DB
}

func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

func (r *Repo) CreateProduct(ctx context.Context, p *domain.Product) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO products (sku, name, description, price)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (sku) DO UPDATE
		SET name = EXCLUDED.name, description = EXCLUDED.description, price = EXCLUDED.price, updated_at = NOW()
	`, p.SKU, p.Name, p.Description, p.Price)
	return err
}

func (r *Repo) GetProduct(ctx context.Context, sku string) (*domain.Product, error) {
	row := r.db.QueryRowContext(ctx, `SELECT sku, name, description, price FROM products WHERE sku = $1`, sku)
	var p domain.Product
	if err := row.Scan(&p.SKU, &p.Name, &p.Description, &p.Price); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *Repo) SearchProducts(ctx context.Context, query string) ([]domain.Product, error) {
	q := "%" + strings.TrimSpace(query) + "%"
	rows, err := r.db.QueryContext(ctx, `
		SELECT sku, name, description, price
		FROM products
		WHERE sku ILIKE $1 OR name ILIKE $1 OR description ILIKE $1
		ORDER BY sku
	`, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func (r *Repo) UpdatePrice(ctx context.Context, sku string, price float64) (bool, error) {
	res, err := r.db.ExecContext(ctx, `UPDATE products SET price = $2, updated_at = NOW() WHERE sku = $1`, sku, price)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (r *Repo) BulkGetProducts(ctx context.Context, skus []string) ([]domain.Product, error) {
	if len(skus) == 0 {
		return []domain.Product{}, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT sku, name, description, price
		FROM products
		WHERE sku = ANY($1)
		ORDER BY sku
	`, pq.Array(skus))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func (r *Repo) DeleteProduct(ctx context.Context, sku string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `DELETE FROM products WHERE sku = $1`, sku)
	if err != nil {
		return false, err
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (r *Repo) ListProducts(ctx context.Context, limit, offset int32) ([]domain.Product, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT sku, name, description, price
		FROM products
		ORDER BY sku
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func (r *Repo) UpsertProduct(ctx context.Context, p *domain.Product) (bool, error) {
	exists, err := r.GetProduct(ctx, p.SKU)
	if err != nil {
		return false, err
	}
	created := exists == nil
	if err := r.CreateProduct(ctx, p); err != nil {
		return false, err
	}
	return created, nil
}

func (r *Repo) BatchUpdatePrice(ctx context.Context, updates []usecases.PriceUpdate) (int32, error) {
	if len(updates) == 0 {
		return 0, nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var updated int32
	for _, u := range updates {
		res, err := tx.ExecContext(ctx, `UPDATE products SET price = $2, updated_at = NOW() WHERE sku = $1`, u.SKU, u.Price)
		if err != nil {
			return 0, err
		}
		affected, _ := res.RowsAffected()
		if affected > 0 {
			updated++
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return updated, nil
}

func (r *Repo) GetProductsByPriceRange(ctx context.Context, minPrice, maxPrice float64) ([]domain.Product, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT sku, name, description, price
		FROM products
		WHERE price BETWEEN $1 AND $2
		ORDER BY sku
	`, minPrice, maxPrice)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanProducts(rows)
}

func (r *Repo) AdjustPriceByPercent(ctx context.Context, sku string, percent float64) (bool, float64, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE products
		SET price = price + (price * $2 / 100.0), updated_at = NOW()
		WHERE sku = $1
		RETURNING price
	`, sku, percent)
	var newPrice float64
	if err := row.Scan(&newPrice); err != nil {
		if err == sql.ErrNoRows {
			return false, 0, nil
		}
		return false, 0, err
	}
	return true, newPrice, nil
}

func (r *Repo) GetCatalogStats(ctx context.Context) (int32, float64, error) {
	row := r.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(AVG(price), 0) FROM products`)
	var total int32
	var avg float64
	if err := row.Scan(&total, &avg); err != nil {
		return 0, 0, err
	}
	return total, avg, nil
}

func scanProducts(rows *sql.Rows) ([]domain.Product, error) {
	out := make([]domain.Product, 0)
	for rows.Next() {
		var p domain.Product
		if err := rows.Scan(&p.SKU, &p.Name, &p.Description, &p.Price); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
