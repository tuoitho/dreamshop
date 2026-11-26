ALTER TABLE orders DROP CONSTRAINT orders_status_check;

ALTER TABLE orders
    ADD CONSTRAINT orders_status_check CHECK (status IN (
        'PENDING_STOCK', 'PENDING_PAYMENT', 'PENDING_CONFIRMATION', 'CONFIRMED',
        'PREPARING', 'SHIPPING', 'DELIVERED', 'DELIVERY_FAILED', 'CANCELLED'));

ALTER TABLE orders
    ADD COLUMN auto_confirm_at TIMESTAMPTZ,
    ADD COLUMN shipping_carrier VARCHAR(100) NOT NULL DEFAULT '',
    ADD COLUMN tracking_number VARCHAR(100) NOT NULL DEFAULT '';

CREATE INDEX ix_orders_pending_confirmation_due
    ON orders (auto_confirm_at)
    WHERE status = 'PENDING_CONFIRMATION';

CREATE INDEX ix_orders_status_created_at
    ON orders (status, created_at DESC);

CREATE TABLE order_status_history (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders (id) ON DELETE CASCADE,
    from_status VARCHAR(32),
    to_status VARCHAR(32) NOT NULL,
    actor_type VARCHAR(16) NOT NULL CHECK (actor_type IN ('SYSTEM', 'CUSTOMER', 'ADMIN')),
    actor_id UUID,
    reason VARCHAR(128) NOT NULL DEFAULT '',
    shipping_carrier VARCHAR(100) NOT NULL DEFAULT '',
    tracking_number VARCHAR(100) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_order_status_history_order_created
    ON order_status_history (order_id, created_at);
