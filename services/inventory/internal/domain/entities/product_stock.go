package entities

import "time"

type ProductStock struct {
	ID               string
	SKU              string
	WarehouseID      string
	TotalQuantity    int64
	ReservedQuantity int64
	SafetyStockLevel int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (p *ProductStock) AvailableQuantity() int64 {
	return p.TotalQuantity - p.ReservedQuantity
}

func (p *ProductStock) Deduct(qty int64) {
	p.TotalQuantity -= qty
	if p.TotalQuantity < 0 {
		p.TotalQuantity = 0
	}
}

func (p *ProductStock) Release(qty int64) {
	p.ReservedQuantity -= qty
	if p.ReservedQuantity < 0 {
		p.ReservedQuantity = 0
	}
}
