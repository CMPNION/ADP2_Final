package usecases

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	catpb "github.com/cmpnion/adp-final/proto/catalog"
	"github.com/cmpnion/adp-final/services/order/internal/domain"
	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
)

type Repository interface {
	CreateOrder(ctx context.Context, o *domain.Order) error
	GetOrder(ctx context.Context, orderID string) (*domain.Order, error)
	UpdateStatus(ctx context.Context, orderID, status string) (bool, error)
	BulkGetOrders(ctx context.Context, orderIDs []string) ([]domain.Order, error)
	ListOrdersByUser(ctx context.Context, userID string) ([]domain.Order, error)
	ListOrdersByStatus(ctx context.Context, status string) ([]domain.Order, error)
	Stats(ctx context.Context) (int32, int32, int32, error)
}

type Service struct {
	repo Repository
	nc   *nats.Conn
	cat  catpb.CatalogServiceClient
}

func NewService(repo Repository, nc *nats.Conn, cat catpb.CatalogServiceClient) *Service {
	return &Service{repo: repo, nc: nc, cat: cat}
}

func (s *Service) CreateOrder(ctx context.Context, userID string, items []domain.OrderItem) (string, error) {
	if strings.TrimSpace(userID) == "" {
		return "", fmt.Errorf("user id is required")
	}
	// items are optional: orders can be created empty and items added later via reserve
	if s.cat == nil && len(items) > 0 {
		return "", fmt.Errorf("catalog client is not configured")
	}
	normalized := make([]domain.OrderItem, 0, len(items))
	skus := make([]string, 0, len(items))
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		sku := strings.TrimSpace(item.SKU)
		if sku == "" {
			return "", fmt.Errorf("sku is required")
		}
		if item.Quantity <= 0 {
			return "", fmt.Errorf("quantity must be greater than zero")
		}
		if _, ok := seen[sku]; !ok {
			seen[sku] = struct{}{}
			skus = append(skus, sku)
		}
		normalized = append(normalized, domain.OrderItem{SKU: sku, Quantity: item.Quantity})
	}
	if len(skus) > 0 {
		resp, err := s.cat.BulkGetProducts(ctx, &catpb.BulkGetProductsRequest{Sku: skus})
		if err != nil {
			return "", err
		}
		products := make(map[string]struct{}, len(resp.GetProducts()))
		for _, p := range resp.GetProducts() {
			products[p.GetSku()] = struct{}{}
		}
		for _, sku := range skus {
			if _, ok := products[sku]; !ok {
				return "", fmt.Errorf("product not found: %s", sku)
			}
		}
	}
	id := uuid.New().String()
	o := &domain.Order{ID: id, UserID: strings.TrimSpace(userID), Items: normalized, Status: domain.OrderStatusCreated.String()}
	if err := s.repo.CreateOrder(ctx, o); err != nil {
		return "", err
	}
	// publish order.created, include items if present
	s.publish("order.created", map[string]any{"order_id": id, "user_id": o.UserID, "items": o.Items})
	log.Printf("order created %s", id)
	return id, nil
}

func (s *Service) CancelOrder(ctx context.Context, orderID string) bool {
	return s.updateStatus(ctx, orderID, domain.OrderStatusCancelled.String())
}

func (s *Service) GetOrder(ctx context.Context, orderID string) (*domain.Order, bool) {
	o, err := s.repo.GetOrder(ctx, orderID)
	if err != nil || o == nil {
		return nil, false
	}
	return o, true
}

func (s *Service) UpdateStatus(ctx context.Context, orderID, status string) bool {
	normalized, ok := domain.NormalizeOrderStatus(status)
	if !ok {
		return false
	}
	return s.updateStatus(ctx, orderID, normalized.String())
}

func (s *Service) CalculateTotal(ctx context.Context, items []domain.OrderItem) float64 {
	if len(items) == 0 || s.cat == nil {
		return 0
	}
	qtyBySKU := make(map[string]int64, len(items))
	skus := make([]string, 0, len(items))
	for _, item := range items {
		sku := strings.TrimSpace(item.SKU)
		if sku == "" || item.Quantity <= 0 {
			continue
		}
		if _, exists := qtyBySKU[sku]; !exists {
			skus = append(skus, sku)
		}
		qtyBySKU[sku] += item.Quantity
	}
	if len(skus) == 0 {
		return 0
	}
	resp, err := s.cat.BulkGetProducts(ctx, &catpb.BulkGetProductsRequest{Sku: skus})
	if err != nil {
		log.Printf("order calculate: catalog bulk get error: %v", err)
		return 0
	}
	priceBySKU := make(map[string]float64, len(resp.GetProducts()))
	for _, p := range resp.GetProducts() {
		priceBySKU[p.GetSku()] = p.GetPrice()
	}
	var total float64
	for sku, qty := range qtyBySKU {
		price, ok := priceBySKU[sku]
		if !ok {
			continue
		}
		total += float64(qty) * price
	}
	return total
}

func (s *Service) BulkGetOrders(ctx context.Context, orderIDs []string) []domain.Order {
	out, err := s.repo.BulkGetOrders(ctx, orderIDs)
	if err != nil {
		return []domain.Order{}
	}
	return out
}

func (s *Service) ListOrdersByUser(ctx context.Context, userID string) []domain.Order {
	out, err := s.repo.ListOrdersByUser(ctx, userID)
	if err != nil {
		return []domain.Order{}
	}
	return out
}

func (s *Service) ListOrdersByStatus(ctx context.Context, status string) []domain.Order {
	out, err := s.repo.ListOrdersByStatus(ctx, strings.ToUpper(strings.TrimSpace(status)))
	if err != nil {
		return []domain.Order{}
	}
	return out
}

func (s *Service) ConfirmOrder(ctx context.Context, orderID string) bool {
	return s.updateStatus(ctx, orderID, domain.OrderStatusConfirmed.String())
}

func (s *Service) MarkOrderPaid(ctx context.Context, orderID string) bool {
	return s.updateStatus(ctx, orderID, domain.OrderStatusPaid.String())
}

func (s *Service) ShipOrder(ctx context.Context, orderID string) bool {
	return s.updateStatus(ctx, orderID, domain.OrderStatusShipped.String())
}

func (s *Service) Stats(ctx context.Context) (int32, int32, int32) {
	total, pending, completed, err := s.repo.Stats(ctx)
	if err != nil {
		return 0, 0, 0
	}
	return total, pending, completed
}

func (s *Service) updateStatus(ctx context.Context, orderID, status string) bool {
	normalized, ok := domain.NormalizeOrderStatus(status)
	if !ok {
		return false
	}
	ok, err := s.repo.UpdateStatus(ctx, orderID, normalized.String())
	if err != nil || !ok {
		return false
	}
	if normalized == domain.OrderStatusCancelled {
		s.publish("order.cancelled", map[string]string{"order_id": orderID})
	} else {
		s.publish("order.updated", map[string]string{"order_id": orderID, "status": normalized.String()})
	}
	return true
}

func (s *Service) publish(subject string, payload any) {
	if s.nc == nil {
		return
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = s.nc.Publish(subject, data)
}
