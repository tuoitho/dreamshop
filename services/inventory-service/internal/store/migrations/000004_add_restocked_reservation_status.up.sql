ALTER TABLE stock_reservations
    DROP CONSTRAINT stock_reservations_status_check;

ALTER TABLE stock_reservations
    ADD CONSTRAINT stock_reservations_status_check
    CHECK (status IN ('ACTIVE', 'CONFIRMED', 'RELEASED', 'RESTOCKED'));
