package order

type OrderItem struct {
	Sku      string
	Quantity int64
}

type Order struct {
	OrderId string
	Items   []OrderItem
}
