package usecases_test

import (
	"context"
	"strings"
	"testing"

	"github.com/cmpnion/adp-final/services/catalog/internal/application/usecases"
	"github.com/cmpnion/adp-final/services/catalog/internal/domain"
)

type fakeRepo struct {
	items map[string]domain.Product
}

func newFakeRepo() *fakeRepo { return &fakeRepo{items: make(map[string]domain.Product)} }

func (r *fakeRepo) CreateProduct(_ context.Context, p *domain.Product) error {
	r.items[p.SKU] = *p
	return nil
}
func (r *fakeRepo) GetProduct(_ context.Context, sku string) (*domain.Product, error) {
	p, ok := r.items[sku]
	if !ok {
		return nil, nil
	}
	cp := p
	return &cp, nil
}
func (r *fakeRepo) SearchProducts(_ context.Context, query string) ([]domain.Product, error) {
	query = strings.ToLower(query)
	out := make([]domain.Product, 0)
	for _, p := range r.items {
		if strings.Contains(strings.ToLower(p.SKU), query) || strings.Contains(strings.ToLower(p.Name), query) {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *fakeRepo) UpdatePrice(_ context.Context, sku string, price float64) (bool, error) {
	p, ok := r.items[sku]
	if !ok {
		return false, nil
	}
	p.Price = price
	r.items[sku] = p
	return true, nil
}
func (r *fakeRepo) BulkGetProducts(_ context.Context, skus []string) ([]domain.Product, error) {
	out := make([]domain.Product, 0)
	for _, sku := range skus {
		if p, ok := r.items[sku]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *fakeRepo) DeleteProduct(_ context.Context, sku string) (bool, error) {
	_, ok := r.items[sku]
	if !ok {
		return false, nil
	}
	delete(r.items, sku)
	return true, nil
}
func (r *fakeRepo) ListProducts(_ context.Context, limit, offset int32) ([]domain.Product, error) {
	all := make([]domain.Product, 0, len(r.items))
	for _, p := range r.items {
		all = append(all, p)
	}
	if offset > int32(len(all)) {
		return []domain.Product{}, nil
	}
	end := offset + limit
	if limit <= 0 || end > int32(len(all)) {
		end = int32(len(all))
	}
	return all[offset:end], nil
}
func (r *fakeRepo) UpsertProduct(ctx context.Context, p *domain.Product) (bool, error) {
	_, exists := r.items[p.SKU]
	if err := r.CreateProduct(ctx, p); err != nil {
		return false, err
	}
	return !exists, nil
}
func (r *fakeRepo) BatchUpdatePrice(ctx context.Context, updates []usecases.PriceUpdate) (int32, error) {
	var n int32
	for _, u := range updates {
		ok, _ := r.UpdatePrice(ctx, u.SKU, u.Price)
		if ok {
			n++
		}
	}
	return n, nil
}
func (r *fakeRepo) GetProductsByPriceRange(_ context.Context, min, max float64) ([]domain.Product, error) {
	out := make([]domain.Product, 0)
	for _, p := range r.items {
		if p.Price >= min && p.Price <= max {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *fakeRepo) AdjustPriceByPercent(_ context.Context, sku string, percent float64) (bool, float64, error) {
	p, ok := r.items[sku]
	if !ok {
		return false, 0, nil
	}
	p.Price = p.Price * (1 + percent/100.0)
	r.items[sku] = p
	return true, p.Price, nil
}
func (r *fakeRepo) GetCatalogStats(_ context.Context) (int32, float64, error) {
	if len(r.items) == 0 {
		return 0, 0, nil
	}
	var sum float64
	for _, p := range r.items {
		sum += p.Price
	}
	return int32(len(r.items)), sum / float64(len(r.items)), nil
}

func TestService_CreateAndGetProduct(t *testing.T) {
	repo := newFakeRepo()
	svc := usecases.NewService(repo, nil)

	if err := svc.CreateProduct(context.Background(), &domain.Product{SKU: "SKU1", Name: "Laptop", Description: "Gaming", Price: 1000}); err != nil {
		t.Fatalf("expected create product ok, got err=%v", err)
	}

	p, found := svc.GetProduct(context.Background(), "SKU1")
	if !found {
		t.Fatalf("expected product found")
	}
	if p.Name != "Laptop" || p.Price != 1000 {
		t.Fatalf("unexpected product %+v", p)
	}
}

func TestService_SearchAndPriceOps(t *testing.T) {
	repo := newFakeRepo()
	svc := usecases.NewService(repo, nil)

	_ = svc.CreateProduct(context.Background(), &domain.Product{SKU: "A", Name: "Alpha", Price: 10})
	_ = svc.CreateProduct(context.Background(), &domain.Product{SKU: "B", Name: "Beta", Price: 20})

	res := svc.SearchProducts(context.Background(), "alp")
	if len(res) != 1 || res[0].SKU != "A" {
		t.Fatalf("unexpected search result %+v", res)
	}

	if !svc.UpdatePrice(context.Background(), "A", 15) {
		t.Fatalf("expected update price true")
	}
	p, _ := svc.GetProduct(context.Background(), "A")
	if p.Price != 15 {
		t.Fatalf("expected updated price 15 got %v", p.Price)
	}

	if updated := svc.BatchUpdatePrice(context.Background(), []usecases.PriceUpdate{{SKU: "A", Price: 17}, {SKU: "B", Price: 21}}); updated != 2 {
		t.Fatalf("expected 2 updates got %d", updated)
	}
}

func TestService_Stats(t *testing.T) {
	repo := newFakeRepo()
	svc := usecases.NewService(repo, nil)

	_ = svc.CreateProduct(context.Background(), &domain.Product{SKU: "A", Name: "Alpha", Price: 10})
	_ = svc.CreateProduct(context.Background(), &domain.Product{SKU: "B", Name: "Beta", Price: 20})

	total, avg := svc.GetCatalogStats(context.Background())
	if total != 2 {
		t.Fatalf("expected total 2 got %d", total)
	}
	if avg != 15 {
		t.Fatalf("expected avg 15 got %v", avg)
	}
}
