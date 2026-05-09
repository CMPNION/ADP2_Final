package grpc

import (
	"context"
	"github.com/yourorg/catalog/internal/application/usecases"
	pb "github.com/yourorg/proto/catalog"
)

type Server struct{ pb.UnimplementedCatalogServiceServer; svc *usecases.Service }

func NewServer(s *usecases.Service) *Server { return &Server{svc:s} }

func (s *Server) CreateProduct(ctx context.Context, req *pb.CreateProductRequest) (*pb.CreateProductResponse, error) {
	p := &usecases.domain.Product{}
	_ = s.svc
	return &pb.CreateProductResponse{Sku: req.Product.Sku}, nil
}
