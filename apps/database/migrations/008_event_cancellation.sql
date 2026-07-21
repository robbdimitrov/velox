-- Adds a terminal CANCELLED status distinct from AVAILABLE/EXPIRED: a seat
-- whose event was cancelled must never look rebookable, which reusing
-- AVAILABLE would imply.
ALTER TABLE projection.seat_snapshots
    DROP CONSTRAINT seat_snapshots_status_check;

ALTER TABLE projection.seat_snapshots
    ADD CONSTRAINT seat_snapshots_status_check
    CHECK (status IN ('AVAILABLE', 'HELD', 'RESERVED', 'TRANSFERRED', 'USED', 'CANCELLED'));

ALTER TABLE projection.wallet_tickets
    DROP CONSTRAINT wallet_tickets_status_check;

ALTER TABLE projection.wallet_tickets
    ADD CONSTRAINT wallet_tickets_status_check
    CHECK (status IN ('ISSUED', 'TRANSFERRED', 'USED', 'UPGRADED', 'CANCELLED'));
