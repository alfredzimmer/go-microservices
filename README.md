# Go Microservices

A Go based microservices architecture for e-commerce application.

- Microservices:
  - Account
  - Catalog
  - Order
  - GraphQL

Microservices communicate with each other through gRPC APIs.

## How to run

```bash
docker-compose up -d
```

Then go to `localhost:8000/playground`.

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
    description: "An Excellent Book by Ron Rash", 
    price: 32.05
  }) {
    id
    name
  }
}
```

### Place an Order
```graphql
mutation {
  createOrder(order: {
    accountId: "ACCOUNT_ID_HERE", 
    products: [{id: "PRODUCT_ID_HERE", quantity: 1}]
  }) {
    id
    totalPrice
    createdAt
  }
}
```

### Query
```graphql
query {
  accounts {
    id
    name
    orders {
      id
      totalPrice
      products {
        name
        price
      }
    }
  }
}
```
