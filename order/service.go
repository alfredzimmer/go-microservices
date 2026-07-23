package order

import (
	"context"
	"time"

	"github.com/segmentio/ksuid"
)

type Service interface {
	PostOrder(ctx context.Context, accountId string, products []OrderedProduct, idempotencyKey string) (*Order, error)
	GetOrdersForAccount(ctx context.Context, accountId string) ([]Order, error)
}

type Order struct {
	Id         string
	CreatedAt  time.Time
	TotalPrice float64
	AccountId  string
	Products   []OrderedProduct
}

type OrderedProduct struct {
	Id          string
	Name        string
	Description string
	Price       float64
	Quantity    uint32
}

type orderService struct {
	repository Repository
}

func NewService(r Repository) Service {
	return &orderService{r}
}

func (s orderService) PostOrder(ctx context.Context, accountId string, products []OrderedProduct, idempotencyKey string) (*Order, error) {
	o := &Order{
		Id:         ksuid.New().String(),
		CreatedAt:  time.Now().UTC(),
		AccountId:  accountId,
		TotalPrice: 0.0,
		Products:   products,
	}

	for _, p := range products {
		o.TotalPrice += p.Price * float64(p.Quantity)
	}
	stored, err := s.repository.PutOrder(ctx, *o, idempotencyKey)
	if err != nil {
		return nil, err
	}
	// On an idempotent replay the repository returns the order from the first
	// call (its id, timestamp and frozen total). Attach this request's fully
	// populated products so the response shape is identical either way.
	stored.Products = products
	return &stored, nil
}

func (s orderService) GetOrdersForAccount(ctx context.Context, accountId string) ([]Order, error) {
	return s.repository.GetOrdersForAccount(ctx, accountId)
}
