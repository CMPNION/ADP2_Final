package domain

type OrderItem struct{ SKU string; Quantity int64 }

type Order struct{ ID string; UserID string; Items []OrderItem; Status string }
