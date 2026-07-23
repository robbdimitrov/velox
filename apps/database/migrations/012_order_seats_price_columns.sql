BEGIN;

ALTER TABLE orders.order_seats
    ADD COLUMN IF NOT EXISTS price_amount_minor BIGINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS currency CHAR(3) NOT NULL DEFAULT 'USD';

ALTER TABLE orders.order_seats
    ADD CONSTRAINT order_seats_price_amount_minor_check
    CHECK (price_amount_minor >= 0);

COMMIT;
