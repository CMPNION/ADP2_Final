package entities

import "time"

type Warehouse struct {
	ID        string
	Name      string
	Location  string
	IsActive  bool
	CreatedAt time.Time
}
