package grpc

import (
	"context"

	pb "github.com/cmpnion/adp-final/proto/order"
	"github.com/cmpnion/adp-final/services/order/internal/application/usecases"
	"github.com/cmpnion/adp-final/services/order/internal/domain"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Server struct {
	pb.UnimplementedOrderServiceServer
	svc *usecases.Service
}

func NewServer(svc *usecases.Service) *Server { return &Server{svc: svc} }

func (s *Server) CreateOrder(ctx context.Context, req *pb.CreateOrderRequest) (*pb.CreateOrderResponse, error) {
	items := fromProtoItems(req.GetItems())
	orderID, err := s.svc.CreateOrder(ctx, req.GetUserId(), items)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	return &pb.CreateOrderResponse{OrderId: orderID}, nil
}

func (s *Server) CancelOrder(ctx context.Context, req *pb.CancelOrderRequest) (*pb.CancelOrderResponse, error) {
	return &pb.CancelOrderResponse{Success: s.svc.CancelOrder(ctx, req.GetOrderId())}, nil
}

func (s *Server) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderResponse, error) {
	o, ok := s.svc.GetOrder(ctx, req.GetOrderId())
	if !ok {
		return &pb.GetOrderResponse{}, nil
	}
	return toProtoOrder(*o), nil
}

func (s *Server) UpdateStatus(ctx context.Context, req *pb.UpdateStatusRequest) (*pb.UpdateStatusResponse, error) {
	return &pb.UpdateStatusResponse{Success: s.svc.UpdateStatus(ctx, req.GetOrderId(), req.GetStatus())}, nil
}

func (s *Server) CalculateTotal(ctx context.Context, req *pb.CalculateTotalRequest) (*pb.CalculateTotalResponse, error) {
	total := s.svc.CalculateTotal(ctx, fromProtoItems(req.GetItems()))
	return &pb.CalculateTotalResponse{Total: total}, nil
}

func (s *Server) BulkGetOrders(ctx context.Context, req *pb.BulkGetOrdersRequest) (*pb.BulkGetOrdersResponse, error) {
	orders := s.svc.BulkGetOrders(ctx, req.GetOrderId())
	return &pb.BulkGetOrdersResponse{Orders: toProtoOrders(orders)}, nil
}

func (s *Server) ListOrdersByUser(ctx context.Context, req *pb.ListOrdersByUserRequest) (*pb.ListOrdersByUserResponse, error) {
	orders := s.svc.ListOrdersByUser(ctx, req.GetUserId())
	return &pb.ListOrdersByUserResponse{Orders: toProtoOrders(orders)}, nil
}

func (s *Server) ListOrdersByStatus(ctx context.Context, req *pb.ListOrdersByStatusRequest) (*pb.ListOrdersByStatusResponse, error) {
	orders := s.svc.ListOrdersByStatus(ctx, req.GetStatus())
	return &pb.ListOrdersByStatusResponse{Orders: toProtoOrders(orders)}, nil
}

func (s *Server) ConfirmOrder(ctx context.Context, req *pb.ConfirmOrderRequest) (*pb.ConfirmOrderResponse, error) {
	return &pb.ConfirmOrderResponse{Success: s.svc.ConfirmOrder(ctx, req.GetOrderId())}, nil
}

func (s *Server) MarkOrderPaid(ctx context.Context, req *pb.MarkOrderPaidRequest) (*pb.MarkOrderPaidResponse, error) {
	return &pb.MarkOrderPaidResponse{Success: s.svc.MarkOrderPaid(ctx, req.GetOrderId())}, nil
}

func (s *Server) ShipOrder(ctx context.Context, req *pb.ShipOrderRequest) (*pb.ShipOrderResponse, error) {
	return &pb.ShipOrderResponse{Success: s.svc.ShipOrder(ctx, req.GetOrderId())}, nil
}

func (s *Server) GetOrderStats(ctx context.Context, _ *pb.GetOrderStatsRequest) (*pb.GetOrderStatsResponse, error) {
	total, pending, completed := s.svc.Stats(ctx)
	return &pb.GetOrderStatsResponse{
		TotalOrders:     total,
		PendingOrders:   pending,
		CompletedOrders: completed,
	}, nil
}

func fromProtoItems(items []*pb.OrderItem) []domain.OrderItem {
	out := make([]domain.OrderItem, 0, len(items))
	for _, item := range items {
		out = append(out, domain.OrderItem{SKU: item.GetSku(), Quantity: item.GetQty()})
	}
	return out
}

func toProtoOrders(orders []domain.Order) []*pb.GetOrderResponse {
	out := make([]*pb.GetOrderResponse, 0, len(orders))
	for _, o := range orders {
		out = append(out, toProtoOrder(o))
	}
	return out
}

func toProtoOrder(o domain.Order) *pb.GetOrderResponse {
	items := make([]*pb.OrderItem, 0, len(o.Items))
	for _, item := range o.Items {
		items = append(items, &pb.OrderItem{Sku: item.SKU, Qty: item.Quantity})
	}
	return &pb.GetOrderResponse{
		OrderId: o.ID,
		UserId:  o.UserID,
		Items:   items,
		Status:  o.Status,
	}
}
