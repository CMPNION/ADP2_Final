package entities

import "time"

type MovementType string

const (
	In          MovementType = "IN"
	Out         MovementType = "OUT"
	Reserve     MovementType = "RESERVE"
	Release     MovementType = "RELEASE"
	Adjust      MovementType = "ADJUST"
	TransferIn  MovementType = "TRANSFER_IN"
	TransferOut MovementType = "TRANSFER_OUT"
)

type StockMovement struct {
	ID          string
	SKU         string
	WarehouseID string
	Type        MovementType
	Quantity    int64
	ReferenceID string
	CreatedAt   time.Time
}
