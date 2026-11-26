ALTER TABLE orders
    DROP CONSTRAINT IF EXISTS orders_payment_status_check;

ALTER TABLE orders
    ADD CONSTRAINT orders_payment_status_check
    CHECK (payment_status IN ('UNPAID', 'PENDING', 'PAID', 'FAILED', 'REFUND_REQUIRED', 'REFUNDED', 'REFUND_FAILED'));

ALTER TABLE orders
    ADD COLUMN refund_trans_id VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN refund_last_error VARCHAR(255) NOT NULL DEFAULT '',
    ADD COLUMN refunded_at TIMESTAMPTZ,
    ADD COLUMN refund_actor_type VARCHAR(16) NOT NULL DEFAULT '' CHECK (refund_actor_type IN ('', 'SYSTEM', 'ADMIN')),
    ADD COLUMN refund_actor_id UUID,
    ADD COLUMN refund_reason VARCHAR(128) NOT NULL DEFAULT '';

UPDATE orders
SET payment_status = 'REFUND_FAILED',
    refund_last_error = 'Đơn cần hoàn tiền phát sinh trước khi bật hoàn tiền tự động',
    updated_at = now()
WHERE status = 'CANCELLED'
  AND payment_method = 'MOMO'
  AND payment_status = 'REFUND_REQUIRED';
