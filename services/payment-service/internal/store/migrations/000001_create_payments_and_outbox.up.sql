CREATE TABLE payments (
    order_id text PRIMARY KEY,
    order_code text NOT NULL DEFAULT '',
    user_id text NOT NULL DEFAULT '',
    method text NOT NULL DEFAULT 'MOMO' CHECK (method IN ('MOMO')),
    status text NOT NULL CHECK (status IN ('INITIATING', 'PENDING', 'SUCCEEDED', 'FAILED', 'CANCELLED', 'REFUND_REQUIRED')),
    amount_minor bigint NOT NULL DEFAULT 0 CHECK (amount_minor >= 0),
    currency text NOT NULL DEFAULT 'VND',
    momo_order_id text NOT NULL,
    momo_request_id text NOT NULL,
    momo_trans_id text NOT NULL DEFAULT '',
    pay_url text NOT NULL DEFAULT '',
    result_code integer,
    result_message text NOT NULL DEFAULT '',
    cancel_reason text NOT NULL DEFAULT '',
    expires_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT uq_payments_momo_order_id UNIQUE (momo_order_id),
    CONSTRAINT uq_payments_momo_request_id UNIQUE (momo_request_id)
);

CREATE TABLE outbox_events (
    id text PRIMARY KEY,
    event_type text NOT NULL,
    routing_key text NOT NULL,
    aggregate_id text NOT NULL,
    payload jsonb NOT NULL,
    attempts integer NOT NULL DEFAULT 0,
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_payment_outbox_pending
    ON outbox_events (next_attempt_at, created_at)
    WHERE published_at IS NULL;
