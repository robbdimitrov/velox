BEGIN;

CREATE TABLE IF NOT EXISTS projection.pending_wallet_ticket_events (
    ticket_id text PRIMARY KEY,
    order_id uuid NOT NULL,
    event_id text NOT NULL,
    section_id text NOT NULL,
    seat_id text NOT NULL,
    aggregate_version integer NOT NULL CHECK (aggregate_version > 0),
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_projection_pending_wallet_ticket_events_order
    ON projection.pending_wallet_ticket_events (order_id, created_at);

COMMIT;
