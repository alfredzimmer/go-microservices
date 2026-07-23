CREATE TABLE IF NOT EXISTS orders (
  id CHAR(27) PRIMARY KEY,
  created_at TIMESTAMP WITH TIME ZONE NOT NULL,
  account_id CHAR(27) NOT NULL,
  total_price MONEY NOT NULL,
  -- Deduplicates retried PostOrder calls: a repeated idempotency key returns
  -- the order stored on the first call instead of inserting a new one.
  idempotency_key TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS order_products (
  order_id CHAR(27) REFERENCES orders (id) ON DELETE CASCADE,
  product_id CHAR(27),
  quantity INT NOT NULL,
  PRIMARY KEY (product_id, order_id)
);
