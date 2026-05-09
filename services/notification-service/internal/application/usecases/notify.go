package usecases

import (
	"context"
	"encoding/json"
	"log"

	"github.com/nats-io/nats.go"
)

type Service struct{ nc *nats.Conn }

func NewService(nc *nats.Conn) *Service { return &Service{nc:nc} }

func (s *Service) Start(ctx context.Context) error {
	_, _ = s.nc.Subscribe("inventory.stock.low", func(m *nats.Msg){ var payload map[string]interface{}; _ = json.Unmarshal(m.Data, &payload); log.Printf("stock low alert: %v", payload) })
	_, _ = s.nc.Subscribe("order.completed", func(m *nats.Msg){ var payload map[string]interface{}; _ = json.Unmarshal(m.Data, &payload); log.Printf("order completed notify: %v", payload) })
	_, _ = s.nc.Subscribe("order.cancelled", func(m *nats.Msg){ var payload map[string]interface{}; _ = json.Unmarshal(m.Data, &payload); log.Printf("order cancelled notify: %v", payload) })
	return nil
}
