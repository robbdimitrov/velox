# Architecture and Consistency Specification

## System Topology

```text
Svelte 5 Client
  | POST /orders, /checkout
  v
Go Order Service
  | ACID write: orders + outbox
  v
PostgreSQL Outbox
  | Debezium or polling publisher
  v
Kafka
  | order.events.v1, inventory.events.v1, payment.events.v1
  v
Rust Inventory Service <-> Append-Only Event Store
  |
  v
Go Projection Workers -> Elasticsearch/MongoDB -> Read API/WebSockets/SSE
```

Each service owns its database. Cross-service joins are forbidden on the write path. Kafka is the append-only integration log for choreography, projections, audit, and replay.

## Frontend UI: Svelte 5

- Use Runes for local reactive primitives: selected seats, filter state, countdown offsets, and WebSocket deltas.
- Use Canvas for individual seat nodes once a section exceeds 1,000 seats; use SVG for low-density sections and semantic outlines.
- Maintain a local `seatVersionById` map. Apply only monotonic updates.
- Route all queries through read APIs or edge-cached endpoints.
- Route all mutations through command endpoints on Go services.

## Go Order Service

Responsibilities:

- Terminate HTTP/gRPC command ingress.
- Validate JWTs, scopes, payload size, idempotency headers, and reservation tokens.
- Use bounded goroutine worker pools for validation and external payment calls.
- Store orders in PostgreSQL with states `PENDING`, `AWAITING_PAYMENT`, `CONFIRMED`, `FAILED`, `EXPIRED`.
- Write outbox rows in the same transaction as order state transitions.

Ingress pipeline:

```text
auth -> rate limit -> schema validation -> idempotency check -> DB transaction -> outbox insert -> response
```

Never publish directly to Kafka from the same request transaction. Kafka publication must flow through the outbox relay.

## Rust Inventory Service

Responsibilities:

- Consume `OrderCreated`, `PaymentConfirmed`, `PaymentFailed`, and timeout control events.
- Validate seat availability through event stream replay or cached stream snapshots.
- Append immutable inventory events with expected stream version.
- Publish resulting events to Kafka after durable append.

Event store rules:

- Stream key: `seat:{event_id}:{section_id}:{seat_id}`.
- Append requires `expected_version`.
- `VersionMismatch` rejects double allocation.
- No mutable seat status table is the source of truth.
- Use Tokio for async Kafka and event-store I/O.
- Keep command validation allocation-bounded: parse once, borrow where possible, avoid cloning payload maps.

Core event types:

```text
SeatReservationHeld
SeatReservationFailed
SeatReservationExpired
SeatTicketPurchased
SeatTicketTransferred
SeatTicketUsed
SeatTicketUpgraded
```

## Storage Profiles

- Inventory: append-only event store, backed by PostgreSQL event table or RocksDB segments with durable WAL.
- Order: PostgreSQL tables for orders, payments, and `outbox_events`.
- Read model: Elasticsearch for search-heavy discovery or MongoDB for document-oriented wallet and seat snapshots.
- Redis: idempotency keys, token buckets, hot layout locks, and short-lived fanout coordination.

## Event Sourcing and CQRS Mutation

Inventory mutations append events only:

```text
OrderCreated -> SeatReservationHeld
hold expires -> SeatReservationExpired
PaymentConfirmed -> SeatTicketPurchased
PaymentFailed -> SeatReservationExpired
```

Projection workers consume Kafka and flatten immutable facts into read documents:

```text
inventory.events.v1 -> seat_snapshot[event_id, seat_id]
inventory.events.v1 -> wallet_ticket[ticket_id]
order.events.v1     -> order_summary[user_id, order_id]
```

Projection writes must be idempotent by `event_id`. Store the last applied event version per aggregate.

## Choreographed Saga Lifecycle

Successful path:

```text
1. Client POST /orders idempotency_key=K seat=A-12
2. Order Service inserts order PENDING and outbox OrderCreated
3. Outbox relay publishes OrderCreated to Kafka
4. Inventory consumes OrderCreated
5. Inventory appends SeatReservationHeld expected_version=N
6. Inventory publishes SeatReservationHeld
7. Projection updates seat as HELD and WebSocket broadcasts
8. Client POST /checkout idempotency_key=K2 reservation_id=R
9. Payment succeeds and PaymentConfirmed is published
10. Inventory appends SeatTicketPurchased
11. Projection marks seat SOLD and wallet ticket ISSUED
12. Order Service observes PaymentConfirmed and marks CONFIRMED
```

Payment rejection path:

```text
PaymentFailed -> Inventory appends SeatReservationExpired -> Projection marks AVAILABLE -> Order marks FAILED
```

Timeout path:

```text
reservation deadline reached -> expiry scheduler emits ReservationTimeoutDue
Inventory appends SeatReservationExpired if not purchased
Projection marks AVAILABLE
Order marks EXPIRED
```

Every event must carry `event_id`, `correlation_id`, `causation_id`, `aggregate_id`, `schema_version`, `occurred_at`, and `signature`.

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

The frontend generates UUIDv7 or UUIDv4 keys for reserve and checkout commands.

Redis key format:

```text
idem:{service}:{user_id}:{idempotency_key}
```

Processing:

1. `SETNX key request_hash ttl=24h`.
2. If inserted, process command and store response pointer.
3. If key exists with same request hash, return the original result.
4. If key exists with a different hash, return `409 IdempotencyKeyConflict`.

Payment providers must receive the same idempotency key to prevent duplicate charges.

## Zero-Trust Security and Rate Limiting

- Validate JWT issuer, audience, expiry, subject, and scopes at ingress.
- Never trust user-supplied price, seat status, expiry, or fee totals.
- Use Redis token buckets per IP, account, device fingerprint, event, and endpoint.
- Apply stricter buckets to `/orders` and `/checkout` than discovery reads.
- Sign Kafka events with service credentials. Consumers verify signature, schema version, and producer identity before applying events.
- Encrypt secrets through deployment secret stores. Do not place secrets in repo files.
- Log correlation IDs, not card data or raw tokens.
