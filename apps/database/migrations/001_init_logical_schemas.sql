BEGIN;

CREATE SCHEMA IF NOT EXISTS orders;
CREATE SCHEMA IF NOT EXISTS inventory;
CREATE SCHEMA IF NOT EXISTS projection;

CREATE TABLE IF NOT EXISTS orders.orders (
    id uuid PRIMARY KEY,
    user_id text NOT NULL,
    status text NOT NULL CHECK (status IN ('PENDING', 'AWAITING_PAYMENT', 'CONFIRMED', 'FAILED', 'EXPIRED')),
    idempotency_key text NOT NULL,
    request_hash bytea NOT NULL,
    reservation_id text,
    reservation_expires_at timestamptz,
    currency char(3) NOT NULL DEFAULT 'USD',
    total_amount_minor bigint NOT NULL DEFAULT 0 CHECK (total_amount_minor >= 0),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_orders_user_created_at
    ON orders.orders (user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS orders.order_seats (
    order_id uuid NOT NULL REFERENCES orders.orders (id) ON DELETE CASCADE,
    event_id text NOT NULL,
    section_id text NOT NULL,
    seat_id text NOT NULL,
    price_amount_minor bigint NOT NULL CHECK (price_amount_minor >= 0),
    currency char(3) NOT NULL DEFAULT 'USD',
    PRIMARY KEY (order_id, event_id, section_id, seat_id)
);

CREATE TABLE IF NOT EXISTS orders.payments (
    id uuid PRIMARY KEY,
    order_id uuid NOT NULL REFERENCES orders.orders (id) ON DELETE CASCADE,
    provider text NOT NULL,
    provider_payment_id text,
    status text NOT NULL CHECK (status IN ('PENDING', 'CONFIRMED', 'FAILED')),
    idempotency_key text NOT NULL,
    amount_minor bigint NOT NULL CHECK (amount_minor >= 0),
    currency char(3) NOT NULL DEFAULT 'USD',
    failure_code text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, idempotency_key)
);

CREATE TABLE IF NOT EXISTS orders.idempotency_keys (
    service text NOT NULL,
    user_id text NOT NULL,
    idempotency_key text NOT NULL,
    request_hash bytea NOT NULL,
    response_ref text,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (service, user_id, idempotency_key)
);

CREATE INDEX IF NOT EXISTS idx_orders_idempotency_expires_at
    ON orders.idempotency_keys (expires_at);

CREATE TABLE IF NOT EXISTS orders.outbox_events (
    id uuid PRIMARY KEY,
    aggregate_type text NOT NULL,
    aggregate_id text NOT NULL,
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    headers jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    published_at timestamptz,
    publish_attempts integer NOT NULL DEFAULT 0 CHECK (publish_attempts >= 0),
    last_error text
);

CREATE INDEX IF NOT EXISTS idx_orders_outbox_unpublished
    ON orders.outbox_events (created_at, id)
    WHERE published_at IS NULL;

CREATE TABLE IF NOT EXISTS inventory.event_streams (
    stream_key text PRIMARY KEY,
    event_id text NOT NULL,
    section_id text NOT NULL,
    seat_id text NOT NULL,
    current_version integer NOT NULL DEFAULT 0 CHECK (current_version >= 0),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS inventory.events (
    id uuid PRIMARY KEY,
    stream_key text NOT NULL REFERENCES inventory.event_streams (stream_key) ON DELETE CASCADE,
    aggregate_version integer NOT NULL CHECK (aggregate_version > 0),
    event_type text NOT NULL,
    payload jsonb NOT NULL,
    metadata jsonb NOT NULL,
    correlation_id text NOT NULL,
    causation_id text,
    signature bytea NOT NULL,
    occurred_at timestamptz NOT NULL,
    appended_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (stream_key, aggregate_version)
);

CREATE INDEX IF NOT EXISTS idx_inventory_events_correlation_id
    ON inventory.events (correlation_id);

CREATE INDEX IF NOT EXISTS idx_inventory_events_occurred_at
    ON inventory.events (occurred_at);

CREATE TABLE IF NOT EXISTS inventory.reservations (
    reservation_id text PRIMARY KEY,
    order_id uuid NOT NULL,
    user_id text NOT NULL,
    status text NOT NULL CHECK (status IN ('HELD', 'EXPIRED', 'CONFIRMED')),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_inventory_reservations_expiry
    ON inventory.reservations (expires_at)
    WHERE status = 'HELD';

CREATE TABLE IF NOT EXISTS projection.processed_events (
    event_id uuid PRIMARY KEY,
    aggregate_id text NOT NULL,
    aggregate_version integer NOT NULL CHECK (aggregate_version > 0),
    source_topic text NOT NULL,
    source_partition integer NOT NULL,
    source_offset bigint NOT NULL,
    processed_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (source_topic, source_partition, source_offset)
);

CREATE TABLE IF NOT EXISTS projection.seat_snapshots (
    event_id text NOT NULL,
    section_id text NOT NULL,
    seat_id text NOT NULL,
    status text NOT NULL CHECK (status IN ('AVAILABLE', 'HELD', 'SOLD', 'TRANSFERRED', 'USED')),
    aggregate_version integer NOT NULL CHECK (aggregate_version >= 0),
    price_amount_minor bigint NOT NULL DEFAULT 0 CHECK (price_amount_minor >= 0),
    reservation_id text,
    held_by_user_id text,
    expires_at timestamptz,
    ticket_id text,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (event_id, section_id, seat_id)
);

ALTER TABLE projection.seat_snapshots
    ADD COLUMN IF NOT EXISTS price_amount_minor bigint NOT NULL DEFAULT 0 CHECK (price_amount_minor >= 0);

CREATE INDEX IF NOT EXISTS idx_projection_seat_snapshots_section
    ON projection.seat_snapshots (event_id, section_id, status);

CREATE TABLE IF NOT EXISTS projection.order_summaries (
    order_id uuid PRIMARY KEY,
    user_id text NOT NULL,
    status text NOT NULL,
    total_amount_minor bigint NOT NULL CHECK (total_amount_minor >= 0),
    currency char(3) NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_projection_order_summaries_user
    ON projection.order_summaries (user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS projection.wallet_tickets (
    ticket_id text PRIMARY KEY,
    user_id text NOT NULL,
    order_id uuid NOT NULL,
    event_id text NOT NULL,
    section_id text NOT NULL,
    seat_id text NOT NULL,
    status text NOT NULL CHECK (status IN ('ISSUED', 'TRANSFERRED', 'USED', 'UPGRADED')),
    aggregate_version integer NOT NULL CHECK (aggregate_version > 0),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_projection_wallet_tickets_user
    ON projection.wallet_tickets (user_id, updated_at DESC);

COMMIT;
