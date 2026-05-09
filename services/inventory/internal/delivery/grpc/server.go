package grpc

import (
	"context"
	pb "github.com/yourorg/inventory/proto"
	"github.com/yourorg/inventory/internal/application/usecases"
)

type Server struct{ pb.UnimplementedInventoryServer; reserve *usecases.ReserveService }

func NewServer(reserve *usecases.ReserveService) *Server { return &Server{reserve: reserve} }

func (s *Server) ReserveStock(ctx context.Context, req *pb.ReserveStockRequest) (*pb.ReserveStockResponse, error) {
	items := []usecases.ItemReq{}
	for _, it := range req.Items { items = append(items, usecases.ItemReq{SKU: it.Sku, WarehouseID: fmt.Sprintf("%d", it.WarehouseId), Quantity: it.Qty}) }
	res, err := s.reserve.ReserveStock(ctx, req.OrderId, items)
	if err != nil { return &pb.ReserveStockResponse{Success:false, Message: err.Error()}, nil }
	ids := make([]string, 0, len(res))
	for _, r := range res { ids = append(ids, r.ReservationID) }
	return &pb.ReserveStockResponse{Success:true, Message: strings.Join(ids, ",")}, nil
}
