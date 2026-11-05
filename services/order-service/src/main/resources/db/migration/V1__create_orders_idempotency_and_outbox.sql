CREATE TABLE orders (
    id UUID PRIMARY KEY,
    order_code VARCHAR(32) NOT NULL UNIQUE,
    user_id UUID NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    status VARCHAR(32) NOT NULL CHECK (status IN ('PENDING_STOCK', 'PENDING_PAYMENT', 'CONFIRMED', 'CANCELLED')),
    payment_method VARCHAR(16) NOT NULL CHECK (payment_method IN ('COD', 'MOMO')),
    payment_status VARCHAR(32) NOT NULL CHECK (payment_status IN ('UNPAID', 'PENDING', 'PAID', 'FAILED', 'REFUND_REQUIRED')),
    recipient_name VARCHAR(255) NOT NULL,
    recipient_phone VARCHAR(32) NOT NULL,
    province_code VARCHAR(32) NOT NULL,
    province_name VARCHAR(128) NOT NULL,
    ward_code VARCHAR(32) NOT NULL,
    ward_name VARCHAR(128) NOT NULL,
    address_line VARCHAR(512) NOT NULL,
    address_label VARCHAR(64) NOT NULL DEFAULT '',
    currency VARCHAR(8) NOT NULL DEFAULT 'VND',
    subtotal_minor BIGINT NOT NULL CHECK (subtotal_minor >= 0),
    shipping_fee_minor BIGINT NOT NULL CHECK (shipping_fee_minor >= 0),
    total_amount_minor BIGINT NOT NULL CHECK (total_amount_minor >= 0),
    pay_url TEXT NOT NULL DEFAULT '',
    payment_trans_id VARCHAR(64) NOT NULL DEFAULT '',
    payment_expires_at TIMESTAMPTZ,
    note TEXT NOT NULL DEFAULT '',
    cancel_reason VARCHAR(128) NOT NULL DEFAULT '',
    version BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_orders_user_idempotency UNIQUE (user_id, idempotency_key)
);

CREATE INDEX ix_orders_user_created_at
    ON orders (user_id, created_at DESC);

CREATE INDEX ix_orders_pending_stock_stale
    ON orders (created_at)
    WHERE status = 'PENDING_STOCK';

CREATE INDEX ix_orders_pending_payment_exp
    ON orders (payment_expires_at)
    WHERE status = 'PENDING_PAYMENT';

CREATE TABLE order_items (
    id UUID PRIMARY KEY,
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    cart_item_id VARCHAR(64) NOT NULL DEFAULT '',
    product_id VARCHAR(128) NOT NULL,
    sku_id VARCHAR(128) NOT NULL,
    product_version BIGINT NOT NULL DEFAULT 0,
    product_name VARCHAR(255) NOT NULL,
    brand VARCHAR(128) NOT NULL DEFAULT '',
    category_name VARCHAR(128) NOT NULL DEFAULT '',
    variant_label VARCHAR(128) NOT NULL DEFAULT '',
    variant_color_hex VARCHAR(32) NOT NULL DEFAULT '',
    image_url TEXT NOT NULL DEFAULT '',
    image_emoji VARCHAR(32) NOT NULL DEFAULT '',
    image_hue VARCHAR(64) NOT NULL DEFAULT '',
    unit_price_minor BIGINT NOT NULL CHECK (unit_price_minor >= 0),
    quantity INT NOT NULL CHECK (quantity > 0),
    line_total_minor BIGINT NOT NULL CHECK (line_total_minor >= 0)
);

CREATE INDEX ix_order_items_order_id
    ON order_items (order_id);

CREATE TABLE idempotency_records (
    user_id UUID NOT NULL,
    idempotency_key VARCHAR(128) NOT NULL,
    request_fingerprint VARCHAR(64) NOT NULL,
    status VARCHAR(24) NOT NULL CHECK (status IN ('IN_PROGRESS', 'COMPLETED', 'FAILED_TERMINAL')),
    order_id UUID,
    response_json JSONB,
    failure_code VARCHAR(64) NOT NULL DEFAULT '',
    failure_message TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, idempotency_key)
);

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    event_type VARCHAR(64) NOT NULL,
    routing_key VARCHAR(64) NOT NULL,
    event_version INT NOT NULL DEFAULT 1,
    aggregate_id VARCHAR(64) NOT NULL,
    correlation_id VARCHAR(64) NOT NULL DEFAULT '',
    payload JSONB NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,
    last_error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX ix_order_outbox_pending
    ON outbox_events (next_attempt_at, created_at)
    WHERE published_at IS NULL;

CREATE TABLE processed_events (
    event_id VARCHAR(64) PRIMARY KEY,
    event_type VARCHAR(64) NOT NULL,
    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
