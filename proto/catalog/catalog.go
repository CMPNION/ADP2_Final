package catalog

import "context"

type Product struct {
	Sku         string
	Name        string
	Description string
	Price       float64
}

type CreateProductRequest struct {
	Product *Product
}

type CreateProductResponse struct {
	Sku string
}

type CatalogServiceServer interface {
	CreateProduct(context.Context, *CreateProductRequest) (*CreateProductResponse, error)
}

type UnimplementedCatalogServiceServer struct{}

func RegisterCatalogServiceServer(any, CatalogServiceServer) {}
