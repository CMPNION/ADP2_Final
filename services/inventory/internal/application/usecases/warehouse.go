package usecases

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/cmpnion/adp-final/services/inventory/internal/domain/entities"
)

func (s *ReserveService) CreateWarehouse(ctx context.Context, name string, location string) (string, error) {
	w := &entities.Warehouse{ID: uuid.New().String(), Name: name, Location: location, IsActive: true, CreatedAt: time.Now()}
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()
	if err := s.repo.CreateWarehouse(ctx, tx, w); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return w.ID, nil
}

func (s *ReserveService) UpdateWarehouse(ctx context.Context, id string, name string, location string, active bool) error {
	w := &entities.Warehouse{ID: id, Name: name, Location: location, IsActive: active}
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.repo.UpdateWarehouse(ctx, tx, w); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *ReserveService) ListWarehouses(ctx context.Context) ([]*entities.Warehouse, error) {
	return s.repo.ListWarehouses(ctx)
}
