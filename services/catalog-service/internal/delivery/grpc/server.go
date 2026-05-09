package grpc

import (
	"context"
	"omnichannel/catalog/internal/application/usecases"
	"omnichannel/catalog/internal/domain"
	pb "omnichannel/proto/catalog"
)

type Server struct {
	pb.UnimplementedCatalogServiceServer
	svc *usecases.Service
}

func NewServer(s *usecases.Service) *Server { return &Server{svc: s} }

func (s *Server) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.CreateProductResponse, error) {
	if req != nil && req.Product != nil {
		_ = s.svc.CreateProduct(ctx, &domain.Product{SKU: req.Product.Sku, Name: req.Product.Name, Description: req.Product.Description, Price: req.Product.Price})
	}
	return &pb.CreateProductResponse{Sku: req.Product.Sku}, nil
}
