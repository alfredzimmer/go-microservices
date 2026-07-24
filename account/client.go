package account

import (
	context "context"

	"github.com/alfredzimmer/go-microservices/account/pb"
	"github.com/alfredzimmer/go-microservices/tlsconfig"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

type Client struct {
	conn    *grpc.ClientConn
	service pb.AccountServiceClient
}

func NewClient(url string) (*Client, error) {
	tlsOpt, err := tlsconfig.ClientCredentials()
	if err != nil {
		return nil, err
	}
	conn, err := grpc.NewClient(url, tlsOpt, grpc.WithStatsHandler(otelgrpc.NewClientHandler()))
	if err != nil {
		return nil, err
	}

	c := pb.NewAccountServiceClient(conn)
	return &Client{conn, c}, nil
}

func (c *Client) Close() {
	c.conn.Close()
}

func (c *Client) PostAccount(ctx context.Context, name string) (*Account, error) {
	r, err := c.service.PostAccount(
		ctx,
		&pb.PostAccountRequest{Name: name},
	)

	if err != nil {
		return nil, err
	}

	return &Account{
		Id:   r.Account.Id,
		Name: r.Account.Name,
	}, nil
}

func (c *Client) GetAccount(ctx context.Context, id string) (*Account, error) {
	r, err := c.service.GetAccount(
		ctx,
		&pb.GetAccountRequest{Id: id},
	)

	if err != nil {
		return nil, err
	}
	return &Account{
		Id:   r.Account.Id,
		Name: r.Account.Name,
	}, nil
}

func (c *Client) GetAccounts(ctx context.Context, skip uint64, take uint64) ([]Account, error) {
	r, err := c.service.GetAccounts(
		ctx,
		&pb.GetAccountsRequest{
			Skip: skip,
			Take: take,
		},
	)

	if err != nil {
		return nil, err
	}
	accounts := []Account{}
	for _, a := range r.Accounts {
		accounts = append(accounts, Account{
			Id:   a.Id,
			Name: a.Name,
		})
	}
	return accounts, nil
}
