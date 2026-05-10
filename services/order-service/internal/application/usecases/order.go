package usecases

import (
	"context"
	"encoding/json"
	"log"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/cmpnion/adp-final/services/order/internal/domain"
)

type Repo interface {
	Create(ctx context.Context, o *domain.Order) error
}

type Service struct {
	repo Repo
	nc   *nats.Conn
}

func NewService(r Repo, nc *nats.Conn) *Service { return &Service{repo: r, nc: nc} }

func (s *Service) CreateOrder(ctx context.Context, userID string, items []domain.OrderItem) (string, error) {
	id := uuid.New().String()
	o := &domain.Order{ID: id, UserID: userID, Items: items, Status: "CREATED"}
	if err := s.repo.Create(ctx, o); err != nil {
		return "", err
	}
	evt := map[string]interface{}{"order_id": id, "user_id": userID, "items": items}
	if payload, _ := json.Marshal(evt); s.nc != nil {
		_ = s.nc.Publish("order.created", payload)
	}
	log.Printf("order created %s", id)
	return id, nil
}
