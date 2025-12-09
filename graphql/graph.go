package main

import (
	"github.com/99designs/gqlgen/graphql"
	"github.com/alfredzimmer/go-microservices/account"
	"github.com/alfredzimmer/go-microservices/catalog"
	"github.com/alfredzimmer/go-microservices/order"
)

type Server struct {
	accountClient *account.Client
	catalogClient *catalog.Client
	orderClient *order.Client
}

func NewGraphQLServer(accountURL string, catalogURL string, orderURL string) (*Server, error) {
	accountClient, err := account.NewClient(accountURL)

	if err != nil {
		return nil, err
	}
	catalogClient, err := catalog.NewClient(catalogURL)

	if err != nil {
		accountClient.Close()
		return nil, err
	}

	orderClient, err := order.NewClient(orderURL)

	if err != nil {
		accountClient.Close()
		catalogClient.Close()
		return nil, err
	}

	return &Server{
		accountClient,
		catalogClient,
		orderClient,
	}, nil
}


func (s *Server) Mutation() MutationResolver {
	return &mutationResolver{
		server: s,
	}
}

func (s *Server) Query() QueryResolver {
	return &queryResolver{
		server: s,
	}
}


func (s *Server) Account() AccountResolver {
	return &accountResolver{
		server: s,
	}
}

func (s *Server) ToExecutableSchema() graphql.ExecutableSchema  {
	return NewExecutableSchema(
		Config{
			Resolvers: s,
		},
	)
}
