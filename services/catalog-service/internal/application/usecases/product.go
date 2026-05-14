package usecases

import (
	"context"
	"encoding/json"
	"log"

	"github.com/cmpnion/adp-final/services/catalog/internal/domain"
	"github.com/nats-io/nats.go"
)

type PriceUpdate struct {
	SKU   string
	Price float64
}

type Repo interface {
	CreateProduct(ctx context.Context, p *domain.Product) error
	GetProduct(ctx context.Context, sku string) (*domain.Product, error)
	SearchProducts(ctx context.Context, query string) ([]domain.Product, error)
	UpdatePrice(ctx context.Context, sku string, price float64) (bool, error)
	BulkGetProducts(ctx context.Context, skus []string) ([]domain.Product, error)
	DeleteProduct(ctx context.Context, sku string) (bool, error)
	ListProducts(ctx context.Context, limit, offset int32) ([]domain.Product, error)
	UpsertProduct(ctx context.Context, p *domain.Product) (bool, error)
	BatchUpdatePrice(ctx context.Context, updates []PriceUpdate) (int32, error)
	GetProductsByPriceRange(ctx context.Context, minPrice, maxPrice float64) ([]domain.Product, error)
	AdjustPriceByPercent(ctx context.Context, sku string, percent float64) (bool, float64, error)
	GetCatalogStats(ctx context.Context) (int32, float64, error)
}

type Service struct {
	repo Repo
	nats *nats.Conn
}

func NewService(repo Repo, nc *nats.Conn) *Service {
	return &Service{repo: repo, nats: nc}
}

func (s *Service) CreateProduct(ctx context.Context, p *domain.Product) error {
	if p == nil || p.SKU == "" {
		return nil
	}
	if err := s.repo.CreateProduct(ctx, p); err != nil {
		return err
	}
	s.publish("product.created", map[string]string{"sku": p.SKU})
	log.Printf("product created %s", p.SKU)
	return nil
}

func (s *Service) GetProduct(ctx context.Context, sku string) (*domain.Product, bool) {
	p, err := s.repo.GetProduct(ctx, sku)
	if err != nil || p == nil {
		return nil, false
	}
	return p, true
}

func (s *Service) SearchProducts(ctx context.Context, query string) []domain.Product {
	out, err := s.repo.SearchProducts(ctx, query)
	if err != nil {
		log.Printf("catalog search error: %v", err)
		return []domain.Product{}
	}
	return out
}

func (s *Service) UpdatePrice(ctx context.Context, sku string, price float64) bool {
	ok, err := s.repo.UpdatePrice(ctx, sku, price)
	if err != nil {
		log.Printf("catalog update price error: %v", err)
		return false
	}
	if ok {
		s.publish("product.updated", map[string]string{"sku": sku})
	}
	return ok
}

func (s *Service) BulkGetProducts(ctx context.Context, skus []string) []domain.Product {
	out, err := s.repo.BulkGetProducts(ctx, skus)
	if err != nil {
		log.Printf("catalog bulk get error: %v", err)
		return []domain.Product{}
	}
	return out
}

func (s *Service) DeleteProduct(ctx context.Context, sku string) bool {
	ok, err := s.repo.DeleteProduct(ctx, sku)
	if err != nil {
		log.Printf("catalog delete error: %v", err)
		return false
	}
	if ok {
		s.publish("product.deleted", map[string]string{"sku": sku})
	}
	return ok
}

func (s *Service) ListProducts(ctx context.Context, limit, offset int32) []domain.Product {
	out, err := s.repo.ListProducts(ctx, limit, offset)
	if err != nil {
		log.Printf("catalog list error: %v", err)
		return []domain.Product{}
	}
	return out
}

func (s *Service) UpsertProduct(ctx context.Context, p *domain.Product) bool {
	if p == nil || p.SKU == "" {
		return false
	}
	created, err := s.repo.UpsertProduct(ctx, p)
	if err != nil {
		log.Printf("catalog upsert error: %v", err)
		return false
	}
	if created {
		s.publish("product.created", map[string]string{"sku": p.SKU})
		return true
	}
	s.publish("product.updated", map[string]string{"sku": p.SKU})
	return false
}

func (s *Service) BatchUpdatePrice(ctx context.Context, updates []PriceUpdate) int32 {
	updated, err := s.repo.BatchUpdatePrice(ctx, updates)
	if err != nil {
		log.Printf("catalog batch update error: %v", err)
		return 0
	}
	if updated > 0 {
		s.publish("product.batch_updated", map[string]int32{"updated": updated})
	}
	return updated
}

func (s *Service) GetProductsByPriceRange(ctx context.Context, minPrice, maxPrice float64) []domain.Product {
	out, err := s.repo.GetProductsByPriceRange(ctx, minPrice, maxPrice)
	if err != nil {
		log.Printf("catalog range query error: %v", err)
		return []domain.Product{}
	}
	return out
}

func (s *Service) AdjustPriceByPercent(ctx context.Context, sku string, percent float64) (bool, float64) {
	ok, price, err := s.repo.AdjustPriceByPercent(ctx, sku, percent)
	if err != nil {
		log.Printf("catalog adjust error: %v", err)
		return false, 0
	}
	if ok {
		s.publish("product.updated", map[string]string{"sku": sku})
	}
	return ok, price
}

func (s *Service) GetCatalogStats(ctx context.Context) (int32, float64) {
	total, avg, err := s.repo.GetCatalogStats(ctx)
	if err != nil {
		log.Printf("catalog stats error: %v", err)
		return 0, 0
	}
	return total, avg
}

func (s *Service) publish(subject string, payload any) {
	if s.nats == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = s.nats.Publish(subject, data)
}
