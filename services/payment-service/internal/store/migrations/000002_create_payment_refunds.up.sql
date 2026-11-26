ALTER TABLE payments
    DROP CONSTRAINT IF EXISTS payments_status_check;

ALTER TABLE payments
    ADD CONSTRAINT payments_status_check
    CHECK (status IN ('INITIATING', 'PENDING', 'SUCCEEDED', 'FAILED', 'CANCELLED', 'REFUND_REQUIRED', 'REFUNDED', 'REFUND_FAILED'));

CREATE TABLE payment_refunds (
    order_id text PRIMARY KEY REFERENCES payments(order_id) ON DELETE CASCADE,
    refund_order_id text NOT NULL UNIQUE,
    refund_request_id text NOT NULL UNIQUE,
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    currency text NOT NULL DEFAULT 'VND',
    momo_trans_id text NOT NULL DEFAULT '',
    refund_trans_id text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    reason text NOT NULL DEFAULT '',
    rotate_ids_on_next_attempt boolean NOT NULL DEFAULT false,
    status text NOT NULL CHECK (status IN ('PENDING', 'PROCESSING', 'SUCCEEDED', 'FAILED')),
    attempts integer NOT NULL DEFAULT 0,
    max_auto_attempts integer NOT NULL DEFAULT 3,
    next_attempt_at timestamptz,
    result_code integer,
    result_message text NOT NULL DEFAULT '',
    last_error text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX ix_payment_refunds_due
    ON payment_refunds (next_attempt_at, created_at)
    WHERE status IN ('PENDING', 'PROCESSING') AND next_attempt_at IS NOT NULL;

INSERT INTO payment_refunds (
    order_id,
    refund_order_id,
    refund_request_id,
    amount_minor,
    currency,
    momo_trans_id,
    description,
    reason,
    status,
    attempts,
    max_auto_attempts,
    next_attempt_at,
    last_error
)
SELECT
    order_id,
    'REFUND' || upper(md5(order_id)),
    'REFREQ' || substr(upper(md5(order_id || ':req')), 1, 24),
    amount_minor,
    currency,
    momo_trans_id,
    left(
        CASE WHEN order_code <> '' THEN 'Hoan tien don hang ' || order_code
             ELSE 'Hoan tien don hang ' || order_id END,
        255
    ),
    cancel_reason,
    'FAILED',
    3,
    3,
    NULL,
    'Đơn cần hoàn tiền phát sinh trước khi bật hoàn tiền tự động'
FROM payments
WHERE status = 'REFUND_REQUIRED'
  AND amount_minor > 0
  AND momo_trans_id <> ''
ON CONFLICT (order_id) DO NOTHING;

UPDATE payments
SET status = 'REFUND_FAILED',
    updated_at = now()
WHERE status = 'REFUND_REQUIRED'
  AND amount_minor > 0
  AND momo_trans_id <> '';
