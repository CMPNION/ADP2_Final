package usecases

import (
	"context"
	"strings"
)

func (s *ReserveService) warehouseExists(ctx context.Context, warehouseID string) (bool, error) {
	warehouses, err := s.repo.ListWarehouses(ctx)
	if err != nil {
		return false, err
	}
	for _, wh := range warehouses {
		if strings.EqualFold(wh.ID, warehouseID) {
			return true, nil
		}
	}
	return false, nil
}
