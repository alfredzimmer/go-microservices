package order

import (
	"context"
	"database/sql"

	"github.com/lib/pq"
)

type Repository interface {
	Close()
	PutOrder(ctx context.Context, o Order, idempotencyKey string) (Order, error)
	GetOrdersForAccount(ctx context.Context, accountId string) ([]Order, error)
}

type postgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(url string) (Repository, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, err
	}
	err = db.Ping()
	if err != nil {
		return nil, err
	}
	return &postgresRepository{db}, nil
}

func (r *postgresRepository) Close() {
	r.db.Close()
}

func (r *postgresRepository) PutOrder(ctx context.Context, o Order, idempotencyKey string) (stored Order, err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Order{}, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
			return
		}
		err = tx.Commit()
	}()

	res, err := tx.ExecContext(
		ctx,
		`INSERT INTO orders(id, created_at, account_id, total_price, idempotency_key)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (idempotency_key) DO NOTHING`,
		o.Id,
		o.CreatedAt,
		o.AccountId,
		o.TotalPrice,
		idempotencyKey,
	)
	if err != nil {
		return Order{}, err
	}

	// No row inserted means this idempotency key was already used: return the
	// order stored on the first call so a retry never creates a second order.
	if n, _ := res.RowsAffected(); n == 0 {
		existing, ferr := getOrderByIdempotencyKey(ctx, tx, idempotencyKey)
		if ferr != nil {
			err = ferr
			return Order{}, err
		}
		return existing, nil
	}

	stmt, err := tx.PrepareContext(ctx, pq.CopyIn("order_products", "order_id", "product_id", "quantity"))
	if err != nil {
		return Order{}, err
	}
	for _, p := range o.Products {
		if _, err = stmt.ExecContext(ctx, o.Id, p.Id, p.Quantity); err != nil {
			return Order{}, err
		}
	}
	if _, err = stmt.ExecContext(ctx); err != nil {
		return Order{}, err
	}
	if err = stmt.Close(); err != nil {
		return Order{}, err
	}
	return o, nil
}

// getOrderByIdempotencyKey loads the header fields of the order previously
// stored under key. Products are not read back: callers decorate the order
// with catalog data from the current request, the same way the read path does.
func getOrderByIdempotencyKey(ctx context.Context, tx *sql.Tx, key string) (Order, error) {
	var o Order
	err := tx.QueryRowContext(
		ctx,
		`SELECT id, created_at, account_id, total_price::money::numeric::float8
		 FROM orders WHERE idempotency_key = $1`,
		key,
	).Scan(&o.Id, &o.CreatedAt, &o.AccountId, &o.TotalPrice)
	if err != nil {
		return Order{}, err
	}
	return o, nil
}

func (r *postgresRepository) GetOrdersForAccount(ctx context.Context, accountId string) ([]Order, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT
		o.id,
		o.created_at,
		o.account_id,
		o.total_price::money::numeric::float8,
		op.product_id,
		op.quantity
		FROM orders o JOIN order_products op ON(o.id = op.order_id)
		WHERE o.account_id = $1
		ORDER By o.id`,
		accountId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := []Order{}
	order := &Order{}
	lastOrder := &Order{}
	orderedProduct := &OrderedProduct{}
	products := []OrderedProduct{}

	for rows.Next() {
		if err = rows.Scan(
			&order.Id,
			&order.CreatedAt,
			&order.AccountId,
			&order.TotalPrice,
			&orderedProduct.Id,
			&orderedProduct.Quantity,
		); err != nil {
			return nil, err
		}
		if lastOrder.Id != "" && lastOrder.Id != order.Id {
			newOrder := Order{
				Id:         lastOrder.Id,
				AccountId:  lastOrder.AccountId,
				CreatedAt:  lastOrder.CreatedAt,
				TotalPrice: lastOrder.TotalPrice,
				Products:   products,
			}
			orders = append(orders, newOrder)
			products = []OrderedProduct{}
		}

		products = append(products, OrderedProduct{
			Id:       orderedProduct.Id,
			Quantity: orderedProduct.Quantity,
		})

		*lastOrder = *order
	}

	newOrder := Order{
		Id:         lastOrder.Id,
		AccountId:  lastOrder.AccountId,
		CreatedAt:  lastOrder.CreatedAt,
		TotalPrice: lastOrder.TotalPrice,
		Products:   products,
	}
	orders = append(orders, newOrder)

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}
