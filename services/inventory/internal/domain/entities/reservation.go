package entities

import "time"

type ReservationStatus string

const (
	Reserved  ReservationStatus = "RESERVED"
	Confirmed ReservationStatus = "CONFIRMED"
	Released  ReservationStatus = "RELEASED"
)

type StockReservation struct {
	ID          string
	OrderID     string
	SKU         string
	WarehouseID string
	Quantity    int64
	Status      ReservationStatus
	ExpiresAt   time.Time
	CreatedAt   time.Time
}
