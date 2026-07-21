-- Align order status with direct reservation confirm/cancel/expire lifecycle.
ALTER TABLE orders.orders
    DROP CONSTRAINT orders_status_check;

ALTER TABLE orders.orders
    ADD CONSTRAINT orders_status_check
    CHECK (status IN ('PENDING', 'HELD', 'CONFIRMED', 'CANCELLED', 'FAILED', 'EXPIRED'));
