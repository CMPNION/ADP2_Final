package proto

type ReserveStockRequest struct {
	OrderID string
	Items   []ReservationItem
}

type ReservationItem struct {
	SKU      string
	Quantity int32
}

type ReserveStockResponse struct {
	Success bool
	Message string
}
