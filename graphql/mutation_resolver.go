package main

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/alfredzimmer/go-microservices/order"
)

var ErrInvalidParameter = errors.New("Invalid Parameter")

type mutationResolver struct {
	server *Server
}

func (r *mutationResolver) CreateAccount(ctx context.Context, in AccountInput) (*Account, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	a, err := r.server.accountClient.PostAccount(ctx, in.Name)
	if err != nil {
		slog.ErrorContext(ctx, "Error creating account", "error", err)
		return nil, err
	}

	return &Account{
		Id:   a.Id,
		Name: a.Name,
	}, nil
}

func (r *mutationResolver) CreateProduct(ctx context.Context, in ProductInput) (*Product, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	p, err := r.server.catalogClient.PostProduct(ctx, in.Name, in.Description, in.Price)
	if err != nil {
		slog.ErrorContext(ctx, "Error creating product", "error", err)
		return nil, err
	}

	return &Product{
		ID:          p.Id,
		Name:        p.Name,
		Description: p.Description,
		Price:       p.Price,
	}, nil
}

func (r *mutationResolver) CreateOrder(ctx context.Context, in OrderInput) (*Order, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var products []order.OrderedProduct
	for _, p := range in.Products {
		if p.Quantity <= 0 {
			return nil, ErrInvalidParameter
		}

		productDetails, err := r.server.catalogClient.GetProduct(ctx, p.ID)
		if err != nil {
			slog.ErrorContext(ctx, "Error fetching product", "productId", p.ID, "error", err)
			return nil, err
		}

		products = append(products, order.OrderedProduct{
			Id:          productDetails.Id,
			Name:        productDetails.Name,
			Description: productDetails.Description,
			Price:       productDetails.Price,
			Quantity:    uint32(p.Quantity),
		})
	}

	o, err := r.server.orderClient.PostOrder(ctx, in.AccountID, products)
	if err != nil {
		slog.ErrorContext(ctx, "Error creating order", "accountId", in.AccountID, "error", err)
		return nil, err
	}

	return &Order{
		ID:         o.Id,
		CreatedAt:  o.CreatedAt,
		TotalPrice: o.TotalPrice,
	}, nil
}
