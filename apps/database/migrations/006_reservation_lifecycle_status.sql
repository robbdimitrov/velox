-- Replaces the payment-oriented AWAITING_PAYMENT status with a direct
-- reservation confirm/cancel/expire lifecycle (no payment step): HELD replaces
-- AWAITING_PAYMENT, and CANCELLED is added for explicit user cancellation.
ALTER TABLE orders.orders
    DROP CONSTRAINT orders_status_check;

ALTER TABLE orders.orders
    ADD CONSTRAINT orders_status_check
    CHECK (status IN ('PENDING', 'HELD', 'CONFIRMED', 'CANCELLED', 'FAILED', 'EXPIRED'));
