package order

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"

	"github.com/alfredzimmer/go-microservices/account"
	"github.com/alfredzimmer/go-microservices/catalog"
	"github.com/alfredzimmer/go-microservices/order/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type grpcServer struct {
	pb.UnimplementedOrderServiceServer
	service       Service
	accountClient *account.Client
	catalogClient *catalog.Client
}

func ListenGRPC(s Service, accountURL string, catalogURL string, port int) error {
	accountClient, err := account.NewClient(accountURL)
	if err != nil {
		return err
	}

	catalogClient, err := catalog.NewClient(catalogURL)
	if err != nil {
		return err
	}

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		accountClient.Close()
		catalogClient.Close()
		return err
	}
	serv := grpc.NewServer()
	pb.RegisterOrderServiceServer(serv, &grpcServer{
		service: s,
		accountClient: accountClient,
		catalogClient: catalogClient,
	})
	reflection.Register(serv)

	return serv.Serve(lis)
}

func (s *grpcServer) PostOrder(ctx context.Context, r *pb.PostOrderRequest) (*pb.PostOrderResponse, error) {
	_, err := s.accountClient.GetAccount(ctx, r.AccountId)
	if err != nil {
		log.Println("Error getting account: ", err)
	}

	productIds := []string{}
	for _, p := range r.Products {
		productIds = append(productIds, p.ProductId)
	}
	orderedProducts, err :=s.catalogClient.GetProducts(ctx, 0, 0, productIds, "")
	if err != nil {
		log.Println("Error getting products:", err)
		return nil, errors.New("Products not found")
	}

	products := []OrderedProduct{}
	for _, p := range orderedProducts {
		product := OrderedProduct{
			Id: p.Id,
			Quantity: 0,
			Price: p.Price,
			Name: p.Name,
			Description: p.Description,
		}
		for _, rp := range r.Products {
			if rp.ProductId == p.Id {
				product.Quantity = rp.Quantity
				break
			}
		}

		if product.Quantity != 0 {
			products = append(products, product)
		}
	}
	order, err := s.service.PostOrder(ctx, r.AccountId, products)
	if err != nil {
		log.Println("Error posting order", err)
		return nil, errors.New("could not post order")
	}

	orderProto := &pb.Order{
		Id: order.Id,
		AccountId: order.AccountId,
		TotalPrice: order.TotalPrice,
		Products: []*pb.Order_OrderProduct{},
	}
	orderProto.CreatedAt, _ = order.CreatedAt.MarshalBinary()
	for _, p := range order.Products {
		orderProto.Products = append(orderProto.Products, &pb.Order_OrderProduct{
			Id: p.Id,
			Name: p.Name,
			Description: p.Description,
			Price: p.Price,
			Quantity: p.Quantity,
		})
	}
	return &pb.PostOrderResponse{
		Order: orderProto,
	}, nil
}

func (s *grpcServer) GetOrdersForAccount(ctx context.Context, r *pb.GetOrdersForAccountRequest) (*pb.GetOrdersForAccountResponse, error) {
	accountOrders, err := s.service.GetOrdersForAccount(ctx, r.AccountId)
	if err != nil {
		log.Println(err)
		return nil, err
	}

	productIdMap := map[string]bool{}
	for _, o := range accountOrders {
		for _, p := range o.Products {
			productIdMap[p.Id] = true
		}
	}

	productIds := []string{}
	for id := range productIdMap {
		productIds = append(productIds, id)
	}
	products, err := s.catalogClient.GetProducts(ctx, 0, 0, productIds, "")
	if err != nil {
		log.Println("Error getting account products", err)
		return nil, err
	}

	orders := []*pb.Order{}
	for _, o := range accountOrders {
		// Encode order
		op := &pb.Order{
			AccountId:  o.AccountId,
			Id:         o.Id,
			TotalPrice: o.TotalPrice,
			Products:   []*pb.Order_OrderProduct{},
		}
		op.CreatedAt, _ = o.CreatedAt.MarshalBinary()

		// Decorate orders with products
		for _, product := range o.Products {
			// Populate product fields
			for _, p := range products {
				if p.Id == product.Id {
					product.Name = p.Name
					product.Description = p.Description
					product.Price = p.Price
					break
				}
			}

			op.Products = append(op.Products, &pb.Order_OrderProduct{
				Id:          product.Id,
				Name:        product.Name,
				Description: product.Description,
				Price:       product.Price,
				Quantity:    product.Quantity,
			})
		}

		orders = append(orders, op)
	}
	return &pb.GetOrdersForAccountResponse{Orders: orders}, nil
}
