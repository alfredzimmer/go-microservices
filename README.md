# Go Microservices

A Go based microservices architecture for e-commerce application.

It include services for account management, product catalog and order processing. Interservice communications are handled by gRPC. GraphQL serves as the API gateway for the entire microservices. 

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
