package usecases

import "context"

func (s *ReserveService) GetLowStockItems(ctx context.Context, limit int) ([]string, error) {
	stocks, err := s.repo.GetLowStock(ctx, limit)
	if err != nil { return nil, err }
	out := make([]string, 0, len(stocks))
	for _, st := range stocks { out = append(out, st.SKU) }
	return out, nil
}
