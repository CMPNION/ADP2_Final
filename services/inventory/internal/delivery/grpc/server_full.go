package grpc

import (
	"context"
	"fmt"
	"strings"

	"omnichannel/inventory/internal/application/usecases"
	pb "omnichannel/proto/inventory"
)

type Server struct {
	pb.UnimplementedInventoryServer
	reserve *usecases.ReserveService
	release *usecases.ReleaseService
	confirm *usecases.ConfirmService
}

func NewServer(reserve *usecases.ReserveService, release *usecases.ReleaseService, confirm *usecases.ConfirmService) *Server {
	return &Server{reserve: reserve, release: release, confirm: confirm}
}

func (s *Server) ReserveStock(ctx context.Context, req *pb.ReserveStockRequest) (*pb.ReserveStockResponse, error) {
	items := []usecases.ItemReq{}
	for _, it := range req.Items {
		items = append(items, usecases.ItemReq{SKU: it.Sku, WarehouseID: fmt.Sprintf("%d", it.WarehouseId), Quantity: it.Qty})
	}
	res, err := s.reserve.ReserveStock(ctx, req.OrderId, items)
	if err != nil {
		return &pb.ReserveStockResponse{Success: false, Message: err.Error()}, nil
	}
	ids := make([]string, 0, len(res))
	for _, r := range res {
		ids = append(ids, r.ReservationID)
	}
	return &pb.ReserveStockResponse{Success: true, Message: strings.Join(ids, ",")}, nil
}

func (s *Server) ReleaseStock(ctx context.Context, req *pb.ReleaseStockRequest) (*pb.ReleaseStockResponse, error) {
	err := s.release.ReleaseStock(ctx, req.OrderId)
	if err != nil {
		return &pb.ReleaseStockResponse{Success: false}, nil
	}
	return &pb.ReleaseStockResponse{Success: true}, nil
}

func (s *Server) ConfirmStockDeduction(ctx context.Context, req *pb.ConfirmStockDeductionRequest) (*pb.ConfirmStockDeductionResponse, error) {
	err := s.confirm.ConfirmStockDeduction(ctx, req.OrderId)
	if err != nil {
		return &pb.ConfirmStockDeductionResponse{Success: false}, nil
	}
	return &pb.ConfirmStockDeductionResponse{Success: true}, nil
}

// The rest of gRPC methods can delegate to other usecases or repo; provide minimal implementations
func (s *Server) GetStockBySKU(ctx context.Context, req *pb.GetStockBySKURequest) (*pb.GetStockBySKUResponse, error) {
	return &pb.GetStockBySKUResponse{}, nil
}
func (s *Server) ListStocksByWarehouse(ctx context.Context, req *pb.ListStocksByWarehouseRequest) (*pb.ListStocksByWarehouseResponse, error) {
	return &pb.ListStocksByWarehouseResponse{}, nil
}
func (s *Server) AddStockReceipt(ctx context.Context, req *pb.AddStockReceiptRequest) (*pb.AddStockReceiptResponse, error) {
	return &pb.AddStockReceiptResponse{Success: true}, nil
}
func (s *Server) TransferStock(ctx context.Context, req *pb.TransferStockRequest) (*pb.TransferStockResponse, error) {
	return &pb.TransferStockResponse{Success: true}, nil
}
func (s *Server) UpdateSafetyStockLevel(ctx context.Context, req *pb.UpdateSafetyStockLevelRequest) (*pb.UpdateSafetyStockLevelResponse, error) {
	return &pb.UpdateSafetyStockLevelResponse{Success: true}, nil
}
func (s *Server) GetLowStockItems(ctx context.Context, req *pb.GetLowStockItemsRequest) (*pb.GetLowStockItemsResponse, error) {
	return &pb.GetLowStockItemsResponse{}, nil
}
func (s *Server) CreateWarehouse(ctx context.Context, req *pb.CreateWarehouseRequest) (*pb.CreateWarehouseResponse, error) {
	return &pb.CreateWarehouseResponse{Success: true}, nil
}
func (s *Server) UpdateWarehouseInfo(ctx context.Context, req *pb.UpdateWarehouseInfoRequest) (*pb.UpdateWarehouseInfoResponse, error) {
	return &pb.UpdateWarehouseInfoResponse{Success: true}, nil
}
func (s *Server) ListWarehouses(ctx context.Context, req *pb.ListWarehousesRequest) (*pb.ListWarehousesResponse, error) {
	return &pb.ListWarehousesResponse{}, nil
}
