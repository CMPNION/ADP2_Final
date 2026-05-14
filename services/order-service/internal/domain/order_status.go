package domain

import "strings"

type OrderStatus string

const (
	OrderStatusCreated   OrderStatus = "CREATED"
	OrderStatusPending   OrderStatus = "PENDING"
	OrderStatusConfirmed OrderStatus = "CONFIRMED"
	OrderStatusPaid      OrderStatus = "PAID"
	OrderStatusShipped   OrderStatus = "SHIPPED"
	OrderStatusCancelled OrderStatus = "CANCELLED"
	OrderStatusCompleted OrderStatus = "COMPLETED"
)

var AllOrderStatuses = []OrderStatus{
	OrderStatusCreated,
	OrderStatusPending,
	OrderStatusConfirmed,
	OrderStatusPaid,
	OrderStatusShipped,
	OrderStatusCancelled,
	OrderStatusCompleted,
}

func (s OrderStatus) String() string {
	return string(s)
}

func NormalizeOrderStatus(value string) (OrderStatus, bool) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case string(OrderStatusCreated):
		return OrderStatusCreated, true
	case string(OrderStatusPending):
		return OrderStatusPending, true
	case string(OrderStatusConfirmed):
		return OrderStatusConfirmed, true
	case string(OrderStatusPaid):
		return OrderStatusPaid, true
	case string(OrderStatusShipped):
		return OrderStatusShipped, true
	case string(OrderStatusCancelled):
		return OrderStatusCancelled, true
	case string(OrderStatusCompleted):
		return OrderStatusCompleted, true
	default:
		return "", false
	}
}

func OrderStatusStrings() []string {
	out := make([]string, 0, len(AllOrderStatuses))
	for _, s := range AllOrderStatuses {
		out = append(out, s.String())
	}
	return out
}
