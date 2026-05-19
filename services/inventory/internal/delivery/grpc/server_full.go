package grpc

import (
	"context"

	"github.com/cmpnion/adp-final/services/inventory/internal/application/usecases"
	"github.com/cmpnion/adp-final/services/inventory/internal/domain"
	pb "github.com/cmpnion/adp-final/proto/inventory"
)

type Server struct {
	pb.UnimplementedInventoryServiceServer
	repo    domain.Repository
	reserve *usecases.ReserveService
	release *usecases.ReleaseService
	confirm *usecases.ConfirmService
}

func NewServer(repo domain.Repository, reserve *usecases.ReserveService, release *usecases.ReleaseService, confirm *usecases.ConfirmService) *Server {
	return &Server{repo: repo, reserve: reserve, release: release, confirm: confirm}
}

func (s *Server) ReserveStock(ctx context.Context, req *pb.ReserveStockRequest) (*pb.ReserveStockResponse, error) {
	if req.OrderId == "" {
		return &pb.ReserveStockResponse{Success: false, Message: "order_id is required"}, nil
	}
	if len(req.Items) == 0 {
		return &pb.ReserveStockResponse{Success: false, Message: "items are required"}, nil
	}

	// Reserve each item individually (simple 1:1 reservations)
	reservedCount := 0
	var lastErr error
	for _, item := range req.Items {
		_, err := s.reserve.ReserveStock(ctx, req.OrderId, item.Sku, item.WarehouseId, item.Qty)
		if err != nil {
			lastErr = err
			continue
		}
		reservedCount++
	}

	if reservedCount == 0 && lastErr != nil {
		return &pb.ReserveStockResponse{Success: false, Message: lastErr.Error()}, nil
	}
	if reservedCount < len(req.Items) {
		return &pb.ReserveStockResponse{
			Success: true,
			Message: "partially reserved: " + lastErr.Error(),
		}, nil
	}
	return &pb.ReserveStockResponse{Success: true, Message: "all items reserved"}, nil
}

func (s *Server) ReleaseStock(ctx context.Context, req *pb.ReleaseStockRequest) (*pb.ReleaseStockResponse, error) {
	_, err := s.release.ReleaseStock(ctx, req.OrderId)
	if err != nil {
		return &pb.ReleaseStockResponse{Success: false}, nil
	}
	return &pb.ReleaseStockResponse{Success: true}, nil
}

func (s *Server) ConfirmStockDeduction(ctx context.Context, req *pb.ConfirmStockDeductionRequest) (*pb.ConfirmStockDeductionResponse, error) {
	_, err := s.confirm.ConfirmStock(ctx, req.OrderId)
	if err != nil {
		return &pb.ConfirmStockDeductionResponse{Success: false}, nil
	}
	return &pb.ConfirmStockDeductionResponse{Success: true}, nil
}

func (s *Server) GetStockBySKU(ctx context.Context, req *pb.GetStockBySKURequest) (*pb.GetStockBySKUResponse, error) {
	stocks, err := s.repo.ListStocksBySKU(ctx, req.GetSku())
	if err != nil {
		return &pb.GetStockBySKUResponse{}, err
	}
	resp := &pb.GetStockBySKUResponse{Sku: req.GetSku()}
	for _, st := range stocks {
		resp.TotalAvailable += st.AvailableQuantity()
		resp.TotalReserved += st.ReservedQuantity
	}
	return resp, nil
}
func (s *Server) ListStocksByWarehouse(ctx context.Context, req *pb.ListStocksByWarehouseRequest) (*pb.ListStocksByWarehouseResponse, error) {
	stocks, err := s.repo.ListStocksByWarehouse(ctx, req.GetWarehouseId())
	if err != nil {
		return &pb.ListStocksByWarehouseResponse{}, err
	}
	resp := &pb.ListStocksByWarehouseResponse{Stocks: make([]*pb.StockRecord, 0, len(stocks))}
	for _, st := range stocks {
		resp.Stocks = append(resp.Stocks, &pb.StockRecord{
			Sku:               st.SKU,
			WarehouseId:       st.WarehouseID,
			TotalQuantity:     st.TotalQuantity,
			ReservedQuantity:  st.ReservedQuantity,
			AvailableQuantity: st.AvailableQuantity(),
			SafetyStockLevel:  st.SafetyStockLevel,
		})
	}
	return resp, nil
}
func (s *Server) AddStockReceipt(ctx context.Context, req *pb.AddStockReceiptRequest) (*pb.AddStockReceiptResponse, error) {
	if err := s.reserve.AddStockReceipt(ctx, req.GetSku(), req.GetWarehouseId(), req.GetQuantity(), req.GetReceiptId()); err != nil {
		return &pb.AddStockReceiptResponse{Success: false}, err
	}
	return &pb.AddStockReceiptResponse{Success: true}, nil
}
func (s *Server) TransferStock(ctx context.Context, req *pb.TransferStockRequest) (*pb.TransferStockResponse, error) {
	if err := s.reserve.TransferStock(ctx, req.GetSku(), req.GetFromWarehouseId(), req.GetToWarehouseId(), req.GetQuantity(), req.GetReferenceId()); err != nil {
		return &pb.TransferStockResponse{Success: false}, err
	}
	return &pb.TransferStockResponse{Success: true}, nil
}
func (s *Server) UpdateSafetyStockLevel(ctx context.Context, req *pb.UpdateSafetyStockLevelRequest) (*pb.UpdateSafetyStockLevelResponse, error) {
	if err := s.reserve.UpdateSafetyStockLevel(ctx, req.GetSku(), req.GetWarehouseId(), req.GetLevel()); err != nil {
		return &pb.UpdateSafetyStockLevelResponse{Success: false}, err
	}
	return &pb.UpdateSafetyStockLevelResponse{Success: true}, nil
}
func (s *Server) GetLowStockItems(ctx context.Context, req *pb.GetLowStockItemsRequest) (*pb.GetLowStockItemsResponse, error) {
	stocks, err := s.reserve.GetLowStockItems(ctx, int(req.GetLimit()))
	if err != nil {
		return &pb.GetLowStockItemsResponse{}, err
	}
	resp := &pb.GetLowStockItemsResponse{Stocks: make([]*pb.StockRecord, 0, len(stocks))}
	for _, st := range stocks {
		resp.Stocks = append(resp.Stocks, &pb.StockRecord{
			Sku:               st.SKU,
			WarehouseId:       st.WarehouseID,
			TotalQuantity:     st.TotalQuantity,
			ReservedQuantity:  st.ReservedQuantity,
			AvailableQuantity: st.AvailableQuantity(),
			SafetyStockLevel:  st.SafetyStockLevel,
		})
	}
	return resp, nil
}
func (s *Server) CreateWarehouse(ctx context.Context, req *pb.CreateWarehouseRequest) (*pb.CreateWarehouseResponse, error) {
	id, err := s.reserve.CreateWarehouse(ctx, req.GetName(), req.GetLocation())
	if err != nil {
		return &pb.CreateWarehouseResponse{Success: false}, err
	}
	return &pb.CreateWarehouseResponse{Success: true, WarehouseId: id}, nil
}
func (s *Server) UpdateWarehouseInfo(ctx context.Context, req *pb.UpdateWarehouseInfoRequest) (*pb.UpdateWarehouseInfoResponse, error) {
	if err := s.reserve.UpdateWarehouse(ctx, req.GetWarehouseId(), req.GetName(), req.GetLocation(), req.GetIsActive()); err != nil {
		return &pb.UpdateWarehouseInfoResponse{Success: false}, err
	}
	return &pb.UpdateWarehouseInfoResponse{Success: true}, nil
}
func (s *Server) ListWarehouses(ctx context.Context, req *pb.ListWarehousesRequest) (*pb.ListWarehousesResponse, error) {
	warehouses, err := s.reserve.ListWarehouses(ctx)
	if err != nil {
		return &pb.ListWarehousesResponse{}, err
	}
	resp := &pb.ListWarehousesResponse{Warehouses: make([]*pb.WarehouseRecord, 0, len(warehouses))}
	for _, wh := range warehouses {
		resp.Warehouses = append(resp.Warehouses, &pb.WarehouseRecord{
			WarehouseId: wh.ID,
			Name:        wh.Name,
			Location:    wh.Location,
			IsActive:    wh.IsActive,
		})
	}
	return resp, nil
}
