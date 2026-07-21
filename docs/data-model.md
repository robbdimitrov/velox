# Data Model

Velox uses one PostgreSQL instance with isolated logical schemas. Services own
their writes even when another service reads the projection.

## Ownership

| Schema | Write owner | Read owners |
| --- | --- | --- |
| `catalog` | `apigateway` organizer/auth paths and database seeds | `apigateway`, `orderservice`, frontend through read APIs |
| `orders` | `orderservice` | `apigateway`, `viewservice` |
| `inventory` | `seatservice` | `viewservice`, wallet ledger reads |
| `projection` | `viewservice` and seed/bootstrap paths | `apigateway` read APIs |

In-memory gateway state exists only for hermetic tests and demo fallback when no
store is configured. Store-backed behavior is the product path.

## Catalog Schema

### `catalog.users`

Fields: `id text primary key`, `email text unique not null`,
`password_hash text not null`, `role text not null`, `created_at timestamptz`.

Write owner: gateway auth registration. Read owner: gateway authentication and
staff reads. Lifecycle: users are not deleted in the current product.

### `catalog.venues`

Fields: `id text primary key`, `name`, `city`, `address`, `capacity`.

Write owner: organizer venue create. Read owner: organizer venue/event flows.
Current gap: no country or status fields. Store-backed venue creation also
creates a default A/B section template with 80 seats.

### `catalog.venue_sections`

Fields: `venue_id`, `section_id`, `name`, `display_order`, `width`, `height`,
`default_price_amount_minor`.

Primary key: `(venue_id, section_id)`. Write owner: venue create and seed
bootstrap. Read owner: event creation. Lifecycle: copied into event sections
when an event is published.

### `catalog.venue_seats`

Fields: `venue_id references catalog.venues(id)`, `section_id`, `seat_id`,
`row_label`, `seat_number`, `x`, `y`, `accessibility`.
Primary key: `(venue_id, section_id, seat_id)`.

Write owner: seeds today; future venue template editor. Read owner: event
creation, which copies seats into inventory streams and projection snapshots.
The current organizer venue create path generates sections A/B, rows A-D, and
seats 01-10 per row.

### `catalog.user_venues`

Fields: `user_id references catalog.users(id)`,
`venue_id references catalog.venues(id)`, `venue_role`. Primary key:
`(user_id, venue_id)`.

Write owner: venue create assigns `owner`. Read owner: organizer ownership and
staff list. Current gap: staff invitation/assignment endpoint is not
implemented.

### `catalog.events`

Fields: `id text primary key`, `venue_id references catalog.venues(id)`,
`name`, `description`, `category`, `image_key`, `starts_at timestamptz`,
`sale_starts_at timestamptz`, `timezone`, `status`, `status_reason`,
`created_at`, `updated_at`.

Write owner: organizer event create and event cancel. Read owners: discovery,
event detail, orderservice booking gate. Current statuses are `PUBLISHED` and
`CANCELLED`; draft/completed are planned.

Category and image keys are constrained to the allowlists documented in
`docs/api.md`.

### `catalog.event_sections`

Fields: `event_id`, `section_id`, `name`, `display_order`, `width`, `height`,
`price_amount_minor`.

Primary key: `(event_id, section_id)`. Write owner: event creation and seeds.
Read owner: public event detail through `section_ids` today; richer section DTOs
can be exposed without changing the storage owner.

### `catalog.event_announcements`

Fields: `id uuid primary key`, `event_id references catalog.events(id)`,
`organizer_id`, `title`, `body`, `severity`, `created_at`.

Constraints: `severity` in `INFO`, `SCHEDULE_CHANGE`, `CANCELLATION`.
Index: `(event_id, created_at desc)`.

Write owner: organizer announcement create. Read owner: public event page.

## Orders Schema

### `orders.orders`

Fields include `id uuid primary key`, `user_id`, `status`,
`idempotency_key`, `request_hash`, `reservation_id`,
`reservation_expires_at`, `currency`, `total_amount_minor`, timestamps.

Constraints: status in `PENDING`, `HELD`, `CONFIRMED`, `CANCELLED`, `FAILED`,
`EXPIRED`; unique `(user_id, idempotency_key)`.

Indexes: `(user_id, created_at desc)`.

Write owner: orderservice. Read owner: gateway order APIs and projections.

### `orders.order_seats`

Fields: `order_id references orders.orders(id)`, `event_id`, `section_id`,
`seat_id`, `price_amount_minor`, `currency`.

Primary key: `(order_id, event_id, section_id, seat_id)`.

Write owner: orderservice create order. Lifecycle follows parent order.

### `orders.payments`

Exists from the early payment-oriented schema but is not used by the current
reservation-only checkout path. Do not expose payment controls unless Phase 3
chooses and implements a local payment simulator.

### `orders.idempotency_keys`

Fields: `service`, `user_id`, `idempotency_key`, `request_hash`,
`response_ref`, `expires_at`, `created_at`.

Primary key: `(service, user_id, idempotency_key)`. Index: `expires_at`.

Write owner: orderservice. Current TTL is 24 hours.

### `orders.outbox_events`

Fields: `id uuid primary key`, `aggregate_type`, `aggregate_id`,
`event_type`, `payload jsonb`, `headers jsonb`, `created_at`, `published_at`,
`publish_attempts`, `last_error`, `last_attempt_at`.

Index: unpublished rows by `(created_at, id)` where `published_at is null`.

Write owner: orderservice in the same transaction as order state changes.
Relay publishes only after commit and marks `published_at` after broker ack.

## Inventory Schema

### `inventory.event_streams`

Fields: `stream_key primary key`, `event_id`, `section_id`, `seat_id`,
`current_version`, `updated_at`.

Write owner: event creation bootstrap and seatservice expected-version appends.
Stream key format is `seat:{event_id}:{section_id}:{seat_id}`.

### `inventory.events`

Fields: `id uuid primary key`, `stream_key references event_streams`,
`aggregate_version`, `event_type`, `payload jsonb`, `metadata jsonb`,
`correlation_id`, `causation_id`, `signature bytea`, `occurred_at`,
`appended_at`.

Constraints: unique `(stream_key, aggregate_version)`. Indexes:
`correlation_id`, `occurred_at`.

Write owner: seatservice. Read owner: viewservice and wallet ledger reads.

### `inventory.reservations`

Fields: `reservation_id primary key`, `order_id`, `user_id`, `status`,
`expires_at`, timestamps.

Constraints: status in `HELD`, `EXPIRED`, `CONFIRMED`. Index:
held reservations by `expires_at`.

Write owner: seatservice. Current product gap: gateway reservation responses
still derive hold expiry locally instead of returning the seatservice-owned
deadline end to end.

### `inventory.processed_events`

Seatservice dedupe table for consumed events. Fields: `event_id primary key`,
`event_type`, `processed_at`.

## Projection Schema

### `projection.processed_events`

Fields: `event_id uuid primary key`, `aggregate_id`, `aggregate_version`,
`source_topic`, `source_partition`, `source_offset`, `processed_at`.

Unique source offset prevents duplicate application.

### `projection.seat_snapshots`

Fields: `event_id`, `section_id`, `seat_id`, `status`, `aggregate_version`,
`price_amount_minor`, `row_label`, `seat_number`, `x`, `y`, `accessibility`,
`reservation_id`, `held_by_user_id`, `expires_at`, `ticket_id`, `updated_at`.

Primary key: `(event_id, section_id, seat_id)`. Index:
`(event_id, section_id, status)`. Status includes `AVAILABLE`, `HELD`, `SOLD`,
`TRANSFERRED`, `USED`, `CANCELLED`.

Write owner: viewservice and event bootstrap. Read owner: gateway seat and
inventory reads.

### `projection.order_summaries`

Fields: `order_id uuid primary key`, `user_id`, `status`,
`total_amount_minor`, `currency`, `updated_at`, `event_id`.

Indexes: `(user_id, updated_at desc)`, `event_id`.

Write owner: viewservice. Read owner: organizer metrics and future order
summary APIs.

### `projection.wallet_tickets`

Fields: `ticket_id primary key`, `user_id`, `order_id`, `event_id`,
`section_id`, `seat_id`, `status`, `aggregate_version`, `updated_at`.

Index: `(user_id, updated_at desc)`. Status includes `ISSUED`, `TRANSFERRED`,
`USED`, `UPGRADED`, `CANCELLED`.

Write owner: viewservice from confirmed inventory events. If order projection
lags, viewservice buffers the confirmation in
`projection.pending_wallet_ticket_events` and drains it when the order summary
arrives.

### `projection.pending_wallet_ticket_events`

Fields: `ticket_id primary key`, `order_id`, `event_id`, `section_id`,
`seat_id`, `aggregate_version`, `created_at`.

Index: `(order_id, created_at)`. `order_id` is the seat event correlation ID
and references the eventual order summary by value; no foreign key is used
because the row exists specifically while `projection.order_summaries` may be
missing.

Write owner: viewservice. Lifecycle: inserted when
`SeatReservationConfirmed` is processed before the matching order summary;
deleted only after the corresponding wallet ticket row is inserted or updated.
If the seat is cancelled before the order summary arrives, drain creates a
`CANCELLED` wallet ticket so the buyer sees the lifecycle instead of a missing
ticket.
