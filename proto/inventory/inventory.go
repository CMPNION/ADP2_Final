package inventory

import "context"

type ReservationItem struct {
	Sku         string
	WarehouseId int64
	Qty         int64
}

type ReserveStockRequest struct {
	OrderId string
	Items   []*ReservationItem
}

type ReserveStockResponse struct {
	Success bool
	Message string
}

type ReleaseStockRequest struct {
	OrderId string
}

type ReleaseStockResponse struct {
	Success bool
}

type ConfirmStockDeductionRequest struct {
	OrderId string
}

type ConfirmStockDeductionResponse struct {
	Success bool
}

type GetStockBySKURequest struct {
	Sku string
}

type GetStockBySKUResponse struct{}

type ListStocksByWarehouseRequest struct{}

type ListStocksByWarehouseResponse struct{}

type AddStockReceiptRequest struct{}

type AddStockReceiptResponse struct {
	Success bool
}

type TransferStockRequest struct{}

type TransferStockResponse struct {
	Success bool
}

type UpdateSafetyStockLevelRequest struct{}

type UpdateSafetyStockLevelResponse struct {
	Success bool
}

type GetLowStockItemsRequest struct{}

type GetLowStockItemsResponse struct{}

type CreateWarehouseRequest struct{}

type CreateWarehouseResponse struct {
	Success bool
}

type UpdateWarehouseInfoRequest struct{}

type UpdateWarehouseInfoResponse struct {
	Success bool
}

type ListWarehousesRequest struct{}

type ListWarehousesResponse struct{}

type InventoryServer interface {
	ReserveStock(context.Context, *ReserveStockRequest) (*ReserveStockResponse, error)
	ReleaseStock(context.Context, *ReleaseStockRequest) (*ReleaseStockResponse, error)
	ConfirmStockDeduction(context.Context, *ConfirmStockDeductionRequest) (*ConfirmStockDeductionResponse, error)
	GetStockBySKU(context.Context, *GetStockBySKURequest) (*GetStockBySKUResponse, error)
	ListStocksByWarehouse(context.Context, *ListStocksByWarehouseRequest) (*ListStocksByWarehouseResponse, error)
	AddStockReceipt(context.Context, *AddStockReceiptRequest) (*AddStockReceiptResponse, error)
	TransferStock(context.Context, *TransferStockRequest) (*TransferStockResponse, error)
	UpdateSafetyStockLevel(context.Context, *UpdateSafetyStockLevelRequest) (*UpdateSafetyStockLevelResponse, error)
	GetLowStockItems(context.Context, *GetLowStockItemsRequest) (*GetLowStockItemsResponse, error)
	CreateWarehouse(context.Context, *CreateWarehouseRequest) (*CreateWarehouseResponse, error)
	UpdateWarehouseInfo(context.Context, *UpdateWarehouseInfoRequest) (*UpdateWarehouseInfoResponse, error)
	ListWarehouses(context.Context, *ListWarehousesRequest) (*ListWarehousesResponse, error)
}

type UnimplementedInventoryServer struct{}

func RegisterInventoryServer(any, InventoryServer) {}
