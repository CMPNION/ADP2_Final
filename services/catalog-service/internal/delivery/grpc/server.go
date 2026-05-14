package grpc

import (
	"context"

	pb "github.com/cmpnion/adp-final/proto/catalog"
	"github.com/cmpnion/adp-final/services/catalog/internal/application/usecases"
	"github.com/cmpnion/adp-final/services/catalog/internal/domain"
)

type Server struct {
	pb.UnimplementedCatalogServiceServer
	svc *usecases.Service
}

func NewServer(s *usecases.Service) *Server { return &Server{svc: s} }

func (s *Server) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.CreateProductResponse, error) {
	if req == nil || req.Product == nil {
		return &pb.CreateProductResponse{}, nil
	}
	p := fromProtoProduct(req.Product)
	_ = s.svc.CreateProduct(ctx, p)
	return &pb.CreateProductResponse{Sku: p.SKU}, nil
}

func (s *Server) GetProduct(ctx context.Context, req *pb.GetProductRequest) (*pb.GetProductResponse, error) {
	p, ok := s.svc.GetProduct(ctx, req.GetSku())
	if !ok {
		return &pb.GetProductResponse{}, nil
	}
	return &pb.GetProductResponse{Product: toProtoProduct(*p)}, nil
}

func (s *Server) SearchProducts(ctx context.Context, req *pb.SearchProductsRequest) (*pb.SearchProductsResponse, error) {
	products := s.svc.SearchProducts(ctx, req.GetQuery())
	return &pb.SearchProductsResponse{Products: toProtoProducts(products)}, nil
}

func (s *Server) UpdatePrice(ctx context.Context, req *pb.UpdatePriceRequest) (*pb.UpdatePriceResponse, error) {
	ok := s.svc.UpdatePrice(ctx, req.GetSku(), req.GetPrice())
	return &pb.UpdatePriceResponse{Success: ok}, nil
}

func (s *Server) BulkGetProducts(ctx context.Context, req *pb.BulkGetProductsRequest) (*pb.BulkGetProductsResponse, error) {
	products := s.svc.BulkGetProducts(ctx, req.GetSku())
	return &pb.BulkGetProductsResponse{Products: toProtoProducts(products)}, nil
}

func (s *Server) DeleteProduct(ctx context.Context, req *pb.DeleteProductRequest) (*pb.DeleteProductResponse, error) {
	ok := s.svc.DeleteProduct(ctx, req.GetSku())
	return &pb.DeleteProductResponse{Success: ok}, nil
}

func (s *Server) ListProducts(ctx context.Context, req *pb.ListProductsRequest) (*pb.ListProductsResponse, error) {
	products := s.svc.ListProducts(ctx, req.GetLimit(), req.GetOffset())
	return &pb.ListProductsResponse{Products: toProtoProducts(products)}, nil
}

func (s *Server) UpsertProduct(ctx context.Context, req *pb.UpsertProductRequest) (*pb.UpsertProductResponse, error) {
	if req == nil || req.Product == nil {
		return &pb.UpsertProductResponse{Created: false}, nil
	}
	created := s.svc.UpsertProduct(ctx, fromProtoProduct(req.Product))
	return &pb.UpsertProductResponse{Created: created}, nil
}

func (s *Server) BatchUpdatePrice(ctx context.Context, req *pb.BatchUpdatePriceRequest) (*pb.BatchUpdatePriceResponse, error) {
	updates := make([]usecases.PriceUpdate, 0, len(req.GetItems()))
	for _, item := range req.GetItems() {
		updates = append(updates, usecases.PriceUpdate{SKU: item.GetSku(), Price: item.GetPrice()})
	}
	updated := s.svc.BatchUpdatePrice(ctx, updates)
	return &pb.BatchUpdatePriceResponse{Updated: updated}, nil
}

func (s *Server) GetProductsByPriceRange(ctx context.Context, req *pb.GetProductsByPriceRangeRequest) (*pb.GetProductsByPriceRangeResponse, error) {
	products := s.svc.GetProductsByPriceRange(ctx, req.GetMinPrice(), req.GetMaxPrice())
	return &pb.GetProductsByPriceRangeResponse{Products: toProtoProducts(products)}, nil
}

func (s *Server) AdjustPriceByPercent(ctx context.Context, req *pb.AdjustPriceByPercentRequest) (*pb.AdjustPriceByPercentResponse, error) {
	ok, price := s.svc.AdjustPriceByPercent(ctx, req.GetSku(), req.GetPercent())
	return &pb.AdjustPriceByPercentResponse{Success: ok, NewPrice: price}, nil
}

func (s *Server) GetCatalogStats(ctx context.Context, _ *pb.GetCatalogStatsRequest) (*pb.GetCatalogStatsResponse, error) {
	total, avg := s.svc.GetCatalogStats(ctx)
	return &pb.GetCatalogStatsResponse{TotalProducts: total, AveragePrice: avg}, nil
}

func fromProtoProduct(p *pb.Product) *domain.Product {
	return &domain.Product{
		SKU:         p.GetSku(),
		Name:        p.GetName(),
		Description: p.GetDescription(),
		Price:       p.GetPrice(),
	}
}

func toProtoProducts(products []domain.Product) []*pb.Product {
	out := make([]*pb.Product, 0, len(products))
	for _, p := range products {
		out = append(out, toProtoProduct(p))
	}
	return out
}

func toProtoProduct(p domain.Product) *pb.Product {
	return &pb.Product{
		Sku:         p.SKU,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
	}
}
