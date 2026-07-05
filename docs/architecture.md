# Architecture

## System Topology

```text
SvelteKit SSR Client
  | POST /reservations, /reservations/{reservation_id}/confirm
  v
apigateway
  | gRPC
  v
orderservice
  | ACID write: orders + outbox
  v
PostgreSQL Outbox
  | Debezium or polling publisher
  v
Kafka
  | order.events.v1, inventory.events.v1
  v
seatservice <-> Append-Only Event Store
  |
  v
viewservice -> Elasticsearch/MongoDB -> Read API/WebSockets/SSE
```

Each service owns its database. Cross-service joins are forbidden on the write
path. Kafka is the append-only integration log for choreography, projections,
audit, and replay.

## Frontend UI

- Use SvelteKit SSR as the browser-facing application boundary.
- Use Svelte 5 with Runes for selected seats, filter state, countdown offsets,
  and WebSocket deltas.
- Use Tailwind for layout and utility styling.
- Use DaisyUI for accessible, themeable controls where it fits the product UI.
- Use Lucide icons for actions, navigation, and tool buttons.
- Use Canvas for individual seat nodes once a section exceeds 1,000 seats; use
  SVG for low-density sections and semantic outlines.
- Maintain a local `seatVersionById` map. Apply only monotonic updates.
- Route all queries through read APIs or edge-cached endpoints.
- Route all mutations through command endpoints on `apigateway`.

## Go Services: `apigateway` and `orderservice`

`apigateway` responsibilities:

- Terminate HTTP/gRPC command ingress.
- Validate JWTs, scopes, request size, rate limits, and public API schemas.
- Enforce role-specific route boundaries; organizer APIs require an
  authenticated `organizer` role before ownership checks run.
- Map public HTTP errors safely and orchestrate bounded gRPC calls to backend
  services.

`orderservice` responsibilities:

- Validate idempotency headers, reservation tokens, and order command payloads.
- Handle explicit user actions via `POST /reservations/{id}/confirm` and
  `POST /reservations/{id}/cancel`.
- Store orders in PostgreSQL with states `PENDING`, `HELD`, `CONFIRMED`,
  `CANCELLED`, `FAILED`, `EXPIRED`.
- Write outbox rows in the same transaction as order state transitions.

Order command HTTP handlers must cap request bodies at 1 MiB, reject unknown
JSON fields and trailing payload data, require `event_id`, `section_id`,
`idempotency_key`, and `user_id`, and accept only 1 to 8 `seat_ids`. Public
errors are JSON objects with stable codes such as `invalid_json`,
`missing_required_fields`, `invalid_seat_count`, `idempotency_key_conflict`, and
`internal_error`.

Ingress pipeline:

```text
auth -> rate limit -> schema validation -> gRPC -> idempotency check -> DB transaction -> outbox insert -> response
```

Never publish directly to Kafka from the same request transaction. Kafka
publication must flow through the outbox relay.

## Rust Service: `seatservice`

Responsibilities:

- Consume `OrderCreated`, `OrderConfirmed`, and `OrderCancelled` events, and
  run a periodic sweep for its own hold-expiry timeouts.
- Validate seat availability through event stream replay or cached stream
  snapshots.
- Append immutable inventory events with expected stream version.
- Publish resulting events to Kafka after durable append.

Event store rules:

- Stream key: `seat:{event_id}:{section_id}:{seat_id}`.
- Append requires `expected_version`.
- `VersionMismatch` rejects double allocation.
- No mutable seat status table is the source of truth.
- Use Tokio for async Kafka and event-store I/O.
- Keep command validation allocation-bounded: parse once, borrow where possible,
  avoid cloning payload maps.

Core event types:

```text
SeatReservationHeld
SeatReservationFailed
SeatReservationExpired
SeatReservationConfirmed
SeatTicketTransferred
SeatTicketUsed
SeatTicketUpgraded
```

## Storage Profiles

- `seatservice`: append-only event store, backed by PostgreSQL event table or
  RocksDB segments with durable WAL.
- `orderservice`: PostgreSQL tables for orders and `outbox_events`.
- Read model: Elasticsearch for search-heavy discovery or MongoDB for
  document-oriented wallet and seat snapshots.
- Redis: idempotency keys, token buckets, hot layout locks, and short-lived
  fanout coordination.

## Event Sourcing and CQRS Mutation

`seatservice` mutations append events only:

```text
OrderCreated     -> SeatReservationHeld
OrderConfirmed   -> SeatReservationConfirmed
OrderCancelled   -> SeatReservationExpired
hold expires (seatservice sweep, no order trigger) -> SeatReservationExpired
```

`viewservice` workers consume Kafka and flatten immutable facts into read
documents:

```text
inventory.events.v1 -> seat_snapshot[event_id, seat_id]
inventory.events.v1 -> wallet_ticket[ticket_id]
order.events.v1     -> order_summary[user_id, order_id]
```

`viewservice` writes must be idempotent by `event_id`. Store the last applied
event version per aggregate.

## Choreographed Saga Lifecycle

Successful path:

```text
1. Client POST /reservations idempotency_key=K seat=A-12
2. `orderservice` inserts order PENDING and outbox OrderCreated
3. Outbox relay publishes OrderCreated to Kafka
4. `seatservice` consumes OrderCreated
5. `seatservice` appends SeatReservationHeld expected_version=N
6. `seatservice` publishes SeatReservationHeld
7. `viewservice` updates seat as HELD and WebSocket broadcasts
8. `orderservice` consumes SeatReservationHeld and marks the order HELD
9. Client POST /reservations/{reservation_id}/confirm idempotency_key=K2
10. `orderservice` validates the order is HELD, marks it CONFIRMED, and
    writes outbox OrderConfirmed
11. Outbox relay publishes OrderConfirmed to Kafka
12. `seatservice` consumes OrderConfirmed and appends SeatReservationConfirmed
13. `viewservice` marks seat SOLD and issues a wallet ticket ISSUED
```

Cancellation path (explicit user action):

```text
1. Client POST /reservations/{reservation_id}/cancel idempotency_key=K3
2. `orderservice` validates the order is PENDING or HELD, marks it
   CANCELLED, and writes outbox OrderCancelled
3. Outbox relay publishes OrderCancelled to Kafka
4. `seatservice` consumes OrderCancelled and appends SeatReservationExpired
   for the held seat stream(s)
5. `viewservice` marks the seat(s) AVAILABLE again
```

Reservation-failed path (seat unavailable at hold time):

```text
`seatservice` appends SeatReservationFailed -> `viewservice` marks AVAILABLE -> `orderservice` consumes SeatReservationFailed and marks FAILED
```

Timeout path (no confirm or cancel before the hold deadline):

```text
1. `seatservice`'s periodic expiry sweep finds a HELD reservation past its
   deadline with no confirm or cancel
2. `seatservice` appends SeatReservationExpired directly, guarded by
   compare-and-append (only if the latest event is still
   SeatReservationHeld)
3. `viewservice` marks the seat AVAILABLE
4. `orderservice` consumes this same SeatReservationExpired event, marks
   its order row EXPIRED, and writes outbox OrderExpired
5. `viewservice` consumes OrderExpired and updates the order's read-model
   status
```

Self-correction path (confirm loses the race against an in-flight expiry):
if the expiry sweep times out a hold in seatservice's authoritative event
store just before orderservice consumes the resulting SeatReservationExpired,
a confirm can still succeed against orderservice's stale local mirror.
`seatservice`'s compare-and-append guard then finds the stream already
Expired and, instead of appending SeatReservationConfirmed, publishes
SeatReservationConfirmationFailed; `orderservice` consumes it and corrects
the order from CONFIRMED to EXPIRED (guarded to only ever apply to that exact
state), writing the same outbox OrderExpired the timeout path above uses.

Every event must carry `event_id`, `correlation_id`, `causation_id`,
`aggregate_id`, `schema_version`, `occurred_at`, and `signature`.

## Transactional Outbox Pattern

Required PostgreSQL columns:

```text
id UUID primary key
aggregate_type text
aggregate_id text
event_type text
payload jsonb
headers jsonb
created_at timestamptz
published_at timestamptz null
publish_attempts int default 0
last_error text null
```

Rules:

- Insert domain row and outbox row in one ACID transaction.
- Relay publishes unpublished rows to Kafka by primary key order.
- Mark `published_at` only after broker acknowledgement.
- Relay retries with exponential backoff.
- Consumers must tolerate duplicate Kafka messages.

## Idempotency Protocol

The frontend generates UUIDv7 or UUIDv4 keys for reserve and confirmation
commands.

Redis key format:

```text
idem:{service}:{user_id}:{idempotency_key}
```

Processing:

1. `SETNX key request_hash ttl=24h`.
2. If inserted, process command and store response pointer.
3. If key exists with same request hash, return the original result.
4. If key exists with a different hash, return `409 IdempotencyKeyConflict`.

Gateway reservation handling must treat pre-order seat holds as tentative until
the order service returns a valid order response. If the upstream order call
fails, times out, or returns malformed data, the gateway releases any tentative
in-memory hold before responding. Successful reservation responses are cached by
idempotency key so retries return the original order without re-calling the
order service.

## Zero-Trust Security and Rate Limiting

- Validate JWT issuer, audience, expiry, subject, and scopes at ingress.
- Never trust user-supplied price, seat status, expiry, or fee totals.
- Use Redis token buckets per IP, account, device fingerprint, event, and
  endpoint.
- Apply stricter buckets to `/reservations` and reservation confirmation than
  discovery reads.
- Sign Kafka events with service credentials. Consumers verify signature, schema
  version, and producer identity before applying events.
- Encrypt secrets through deployment secret stores. Do not place secrets in repo
  files.
- Log correlation IDs, not raw tokens or other sensitive request data.
