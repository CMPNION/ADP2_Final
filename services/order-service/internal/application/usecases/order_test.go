package usecases

import (
	"context"
	"testing"

	"google.golang.org/grpc"

	catpb "github.com/cmpnion/adp-final/proto/catalog"
	"github.com/cmpnion/adp-final/services/order/internal/domain"
)

type fakeRepo struct {
	orders map[string]domain.Order
}

func newFakeRepo() *fakeRepo { return &fakeRepo{orders: make(map[string]domain.Order)} }

// fakeCatalogServiceClient is a minimal mock for testing; implements all methods but only BulkGetProducts does real work
type fakeCatalogServiceClient struct{}

func (f *fakeCatalogServiceClient) CreateProduct(ctx context.Context, in *catpb.CreateProductRequest, opts ...grpc.CallOption) (*catpb.CreateProductResponse, error) {
	return nil, nil
}
func (f *fakeCatalogServiceClient) GetProduct(ctx context.Context, in *catpb.GetProductRequest, opts ...grpc.CallOption) (*catpb.GetProductResponse, error) {
	return nil, nil
}
func (f *fakeCatalogServiceClient) SearchProducts(ctx context.Context, in *catpb.SearchProductsRequest, opts ...grpc.CallOption) (*catpb.SearchProductsResponse, error) {
	return nil, nil
}
func (f *fakeCatalogServiceClient) UpdatePrice(ctx context.Context, in *catpb.UpdatePriceRequest, opts ...grpc.CallOption) (*catpb.UpdatePriceResponse, error) {
	return nil, nil
}
func (f *fakeCatalogServiceClient) BulkGetProducts(ctx context.Context, in *catpb.BulkGetProductsRequest, opts ...grpc.CallOption) (*catpb.BulkGetProductsResponse, error) {
	products := make([]*catpb.Product, 0)
	for _, sku := range in.GetSku() {
		price := 10.0
		if sku == "SKU-2" {
			price = 20.0
		}
		products = append(products, &catpb.Product{Sku: sku, Price: price})
	}
	return &catpb.BulkGetProductsResponse{Products: products}, nil
}
func (f *fakeCatalogServiceClient) DeleteProduct(ctx context.Context, in *catpb.DeleteProductRequest, opts ...grpc.CallOption) (*catpb.DeleteProductResponse, error) {
	return nil, nil
}
func (f *fakeCatalogServiceClient) ListProducts(ctx context.Context, in *catpb.ListProductsRequest, opts ...grpc.CallOption) (*catpb.ListProductsResponse, error) {
	return nil, nil
}
func (f *fakeCatalogServiceClient) UpsertProduct(ctx context.Context, in *catpb.UpsertProductRequest, opts ...grpc.CallOption) (*catpb.UpsertProductResponse, error) {
	return nil, nil
}
func (f *fakeCatalogServiceClient) BatchUpdatePrice(ctx context.Context, in *catpb.BatchUpdatePriceRequest, opts ...grpc.CallOption) (*catpb.BatchUpdatePriceResponse, error) {
	return nil, nil
}
func (f *fakeCatalogServiceClient) GetProductsByPriceRange(ctx context.Context, in *catpb.GetProductsByPriceRangeRequest, opts ...grpc.CallOption) (*catpb.GetProductsByPriceRangeResponse, error) {
	return nil, nil
}
func (f *fakeCatalogServiceClient) AdjustPriceByPercent(ctx context.Context, in *catpb.AdjustPriceByPercentRequest, opts ...grpc.CallOption) (*catpb.AdjustPriceByPercentResponse, error) {
	return nil, nil
}
func (f *fakeCatalogServiceClient) GetCatalogStats(ctx context.Context, in *catpb.GetCatalogStatsRequest, opts ...grpc.CallOption) (*catpb.GetCatalogStatsResponse, error) {
	return nil, nil
}

func (r *fakeRepo) CreateOrder(_ context.Context, o *domain.Order) error {
	r.orders[o.ID] = *o
	return nil
}
func (r *fakeRepo) GetOrder(_ context.Context, orderID string) (*domain.Order, error) {
	o, ok := r.orders[orderID]
	if !ok {
		return nil, nil
	}
	cp := o
	return &cp, nil
}
func (r *fakeRepo) UpdateStatus(_ context.Context, orderID, status string) (bool, error) {
	o, ok := r.orders[orderID]
	if !ok {
		return false, nil
	}
	o.Status = status
	r.orders[orderID] = o
	return true, nil
}
func (r *fakeRepo) BulkGetOrders(_ context.Context, orderIDs []string) ([]domain.Order, error) {
	out := make([]domain.Order, 0)
	for _, id := range orderIDs {
		if o, ok := r.orders[id]; ok {
			out = append(out, o)
		}
	}
	return out, nil
}
func (r *fakeRepo) ListOrdersByUser(_ context.Context, userID string) ([]domain.Order, error) {
	out := make([]domain.Order, 0)
	for _, o := range r.orders {
		if o.UserID == userID {
			out = append(out, o)
		}
	}
	return out, nil
}
func (r *fakeRepo) ListOrdersByStatus(_ context.Context, status string) ([]domain.Order, error) {
	out := make([]domain.Order, 0)
	for _, o := range r.orders {
		if status == "" || o.Status == status {
			out = append(out, o)
		}
	}
	return out, nil
}
func (r *fakeRepo) Stats(_ context.Context) (int32, int32, int32, error) {
	var total, pending, completed int32
	for _, o := range r.orders {
		total++
		switch o.Status {
		case "CREATED", "PENDING":
			pending++
		case "SHIPPED", "COMPLETED":
			completed++
		}
	}
	return total, pending, completed, nil
}

func TestService_OrderLifecycle(t *testing.T) {
	svc := NewService(newFakeRepo(), nil, &fakeCatalogServiceClient{})
	ctx := context.Background()

	orderID, err := svc.CreateOrder(ctx, "user-1", []domain.OrderItem{{SKU: "SKU-1", Quantity: 2}, {SKU: "SKU-2", Quantity: 1}})
	if err != nil || orderID == "" {
		t.Fatalf("expected CreateOrder success, got id=%q err=%v", orderID, err)
	}

	order, ok := svc.GetOrder(ctx, orderID)
	if !ok || order.UserID != "user-1" {
		t.Fatalf("unexpected GetOrder result: %+v ok=%v", order, ok)
	}

	total := svc.CalculateTotal(ctx, order.Items)
	if total != 40 {
		t.Fatalf("expected total=40 (2*10 + 1*20), got %v", total)
	}

	if ok := svc.UpdateStatus(ctx, orderID, "pending"); !ok {
		t.Fatalf("expected UpdateStatus success")
	}
	if ok := svc.ConfirmOrder(ctx, orderID); !ok {
		t.Fatalf("expected ConfirmOrder success")
	}
	if ok := svc.MarkOrderPaid(ctx, orderID); !ok {
		t.Fatalf("expected MarkOrderPaid success")
	}
	if ok := svc.ShipOrder(ctx, orderID); !ok {
		t.Fatalf("expected ShipOrder success")
	}

	byUser := svc.ListOrdersByUser(ctx, "user-1")
	if len(byUser) != 1 {
		t.Fatalf("expected 1 order by user, got %d", len(byUser))
	}
	byStatus := svc.ListOrdersByStatus(ctx, "SHIPPED")
	if len(byStatus) != 1 {
		t.Fatalf("expected 1 shipped order, got %d", len(byStatus))
	}

	bulk := svc.BulkGetOrders(ctx, []string{orderID, "missing"})
	if len(bulk) != 1 || bulk[0].ID != orderID {
		t.Fatalf("unexpected bulk get result: %+v", bulk)
	}

	totalOrders, pending, completed := svc.Stats(ctx)
	if totalOrders != 1 || pending != 0 || completed != 1 {
		t.Fatalf("unexpected stats total=%d pending=%d completed=%d", totalOrders, pending, completed)
	}

	if ok := svc.CancelOrder(ctx, orderID); !ok {
		t.Fatalf("expected CancelOrder success")
	}
	cancelled := svc.ListOrdersByStatus(ctx, "CANCELLED")
	if len(cancelled) != 1 {
		t.Fatalf("expected cancelled order after cancel")
	}
}

func TestService_CreateOrderValidation(t *testing.T) {
	svc := NewService(newFakeRepo(), nil, &fakeCatalogServiceClient{})
	if _, err := svc.CreateOrder(context.Background(), "", []domain.OrderItem{{SKU: "SKU-1", Quantity: 1}}); err == nil {
		t.Fatalf("expected user id validation error")
	}
	if _, err := svc.CreateOrder(context.Background(), "user-1", []domain.OrderItem{{SKU: "", Quantity: 1}}); err == nil {
		t.Fatalf("expected sku validation error")
	}
	if _, err := svc.CreateOrder(context.Background(), "user-1", []domain.OrderItem{{SKU: "SKU-1", Quantity: 0}}); err == nil {
		t.Fatalf("expected quantity validation error")
	}
}
