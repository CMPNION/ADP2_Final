package usecases

import (
	"context"
	"encoding/json"
	"log"

	"github.com/yourorg/catalog/internal/domain"
	"github.com/nats-io/nats.go"
)

type Repo interface { Create(ctx context.Context, p *domain.Product) error }

type Service struct{ repo Repo; nats *nats.Conn }

func NewService(r Repo, nc *nats.Conn) *Service { return &Service{repo: r, nats: nc} }

func (s *Service) CreateProduct(ctx context.Context, p *domain.Product) error {
	if err := s.repo.Create(ctx, p); err != nil { return err }
	payload, _ := json.Marshal(map[string]string{"sku": p.SKU})
	if s.nats != nil { _ = s.nats.Publish("product.updated", payload) }
	log.Printf("product created %s", p.SKU)
	return nil
}
