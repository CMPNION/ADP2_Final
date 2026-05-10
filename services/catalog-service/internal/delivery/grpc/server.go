package grpc

import (
	"context"
	"github.com/cmpnion/adp-final/services/catalog/internal/application/usecases"
	"github.com/cmpnion/adp-final/services/catalog/internal/domain"
	pb "github.com/cmpnion/adp-final/proto/catalog"
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
