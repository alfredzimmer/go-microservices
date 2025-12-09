package catalog

import (
	"context"

	"github.com/segmentio/ksuid"
)

type Service interface {
	PostProduct(ctx context.Context, name string, descrption string, price float64) (*Product, error)
	GetProductById(ctx context.Context, id string) (*Product, error)
	GetProducts(ctx context.Context, skip uint64, take uint64) ([]Product, error)
	GetProductsByIds(ctx context.Context, ids []string) ([]Product, error)
	SearchProducts(ctx context.Context, query string, skip uint64, take uint64) ([]Product, error)
}

type Product struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Price       float64 `json:"price"`
}

type catalogService struct {
	repository Repository
}

func NewService(r Repository) Service {
	return &catalogService{r}
}

func (s *catalogService) PostProduct(ctx context.Context, name string, descrption string, price float64) (*Product, error) {
	p := &Product{
		Name:        name,
		Description: descrption,
		Price:       price,
		Id:          ksuid.New().String(),
	}

	if err := s.repository.PutProduct(ctx, *p); err != nil {
		return nil, err
	}

	return p, nil
}

func (s *catalogService) GetProductById(ctx context.Context, id string) (*Product, error) {
	return s.repository.GetProductById(ctx, id)
}

func (s *catalogService) GetProducts(ctx context.Context, skip uint64, take uint64) ([]Product, error) {
	if take > 100 || (skip == 0 && take == 0) {
		take = 100
	}
	return s.repository.ListProducts(ctx, skip, take)
}

func (s *catalogService) GetProductsByIds(ctx context.Context, ids []string) ([]Product, error) {
	return s.repository.ListProductsWithIds(ctx, ids)
}

func (s *catalogService) SearchProducts(ctx context.Context, query string, skip uint64, take uint64) ([]Product, error) {
	if take > 100 || (skip == 0 && take == 0) {
		take = 100
	}
	return s.repository.SearchProducts(ctx, query, skip, take)
}
