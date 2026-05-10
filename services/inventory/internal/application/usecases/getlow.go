package usecases

import (
	"context"

	"github.com/cmpnion/adp-final/services/inventory/internal/domain/entities"
)

func (s *ReserveService) GetLowStockItems(ctx context.Context, limit int) ([]*entities.ProductStock, error) {
	return s.repo.GetLowStock(ctx, limit)
}
