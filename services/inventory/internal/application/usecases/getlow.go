package usecases

import (
	"context"

	"omnichannel/inventory/internal/domain/entities"
)

func (s *ReserveService) GetLowStockItems(ctx context.Context, limit int) ([]*entities.ProductStock, error) {
	return s.repo.GetLowStock(ctx, limit)
}
