package events

type OrderItem struct {
	Sku string `json:"sku"`
	Qty int64  `json:"qty"`
}

type OrderCreated struct {
	OrderId string      `json:"order_id"`
	Items   []OrderItem `json:"items"`
}

type OrderCancelled struct {
	OrderId string `json:"order_id"`
}

type OrderCompleted struct {
	OrderId string `json:"order_id"`
}
