# Go Microservices

A Go based microservices architecture for e-commerce application.

It includes services for account management, product catalog and order processing. Interservice communications are handled by gRPC. GraphQL serves as the API gateway for the entire microservices. 

- Microservices:
  - Account
  - Catalog
  - Order
  - GraphQL


## How to run

```bash
docker-compose up -d
```

Then go to `localhost:8000/playground`.

## Observability

### Distributed tracing

All services are instrumented with OpenTelemetry. Every GraphQL request produces
a single trace that follows the request through the gateway and across the gRPC
hops into the account, catalog and order services, and spans are exported to
Jaeger (started automatically by docker-compose).

- Open the Jaeger UI at `http://localhost:16686`
- Pick a service (e.g. `graphql`) and click **Find Traces**
- Open a trace to see the full request tree with per-hop timings

The OTLP endpoint is configured per service with the standard
`OTEL_EXPORTER_OTLP_ENDPOINT` environment variable.

### Structured logging

Services log JSON to stdout via `log/slog`, tagged with the service name. Logs
written inside a request (`slog.ErrorContext(ctx, ...)`) also carry the
`trace_id`/`span_id` of the active trace, so a log line can be looked up
directly in Jaeger:

```bash
docker-compose logs -f order
```

```json
{"time":"...","level":"ERROR","msg":"Error getting account","service":"order","accountId":"...","error":"...","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736","span_id":"00f067aa0ba902b7"}
```

## Sample GraphQL APIs

### Create an Account
```graphql
mutation {
    createAccount(account: {name: "Alfred"}) {
        id
        name
    }
}
```

### Create a Product
```graphql
mutation {
  createProduct(product: {
    name: "Burning Bright:Stories", 
    description: "In Burning Bright, the stories span the years from the Civil War to the present day, and Rash's historical and modern settings are sewn together in a hauntingly beautiful patchwork of suspense and myth, populated by raw and unforgettable characters mined from the landscape of Appalachia.", 
    price: 32.05
  }) {
    id
    name
    price
  }
}
```

### Place an Order
```graphql
mutation {
  createOrder(order: {
    accountId: "ACCOUNT_ID", 
    products: [{id: "PRODUCT_ID", quantity: 1}]
  }) {
    id
    totalPrice
    createdAt
    products {
        name
        price
    }
  }
}
```

### Account Query
```graphql
query {
    accounts(id: "ACCOUNT_ID") {
        name
        orders {
            id
            createdAt
            products
            totalPrice
        }
    }
}
```

### Search Products

```graphql
query {
    products(query: "Burning") {
        name
        description
        price
    }
}
```


### Pagination and Filtering

```graphql
query {
    products(pagination: {skip: 0 take: 5}, query: "Burning") {
        id
        name
        description
        price
    }
}
```
