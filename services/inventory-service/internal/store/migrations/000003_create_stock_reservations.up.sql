CREATE TABLE stock_reservations (
    order_id text PRIMARY KEY,
    status text NOT NULL CHECK (status IN ('ACTIVE', 'CONFIRMED', 'RELEASED')),
    expires_at timestamptz NOT NULL,
    release_reason text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_stock_reservations_active_expires
    ON stock_reservations (expires_at)
    WHERE status = 'ACTIVE';

CREATE TABLE stock_reservation_items (
    order_id text NOT NULL REFERENCES stock_reservations(order_id) ON DELETE CASCADE,
    sku_id text NOT NULL,
    product_id text NOT NULL,
    quantity integer NOT NULL CHECK (quantity > 0),
    PRIMARY KEY (order_id, sku_id)
);
