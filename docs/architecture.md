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
viewservice -> PostgreSQL projection schema -> Read API/SSE
```

Each service owns its logical schema. Cross-service joins are forbidden on the
write path. Kafka is the append-only integration log for choreography,
projections, audit, and replay.

## Frontend UI

- Use SvelteKit SSR as the browser-facing application boundary.
- Use Svelte 5 with Runes for selected seats, filter state, countdown offsets,
  and SSE deltas.
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
- Expose canonical browser API routes without a gateway-native `/api` prefix.
  The SvelteKit frontend owns the browser `/api/*` proxy and strips `/api`
  before forwarding. Existing literal gateway `/api/organizer/*` routes are
  compatibility aliases only.
- Enforce role-specific route boundaries; organizer APIs require an
  authenticated `organizer` role before ownership checks run.
- Map public HTTP errors safely and orchestrate bounded gRPC calls to backend
  services.
- Serve a public, cacheable organizer announcement feed per event
  (`GET /events/{id}/announcements`, `POST
  /organizer/events/{id}/announcements`) for schedule changes and other
  updates. No live push; clients re-fetch the list.
- Handle organizer-initiated whole-event cancellation via `POST
  /organizer/events/{id}/cancel`: mark the event `CANCELLED` in
  `catalog.events`, then call orderservice's `POST /events/{id}/cancel` to
  bulk-cancel every order tied to it. Both steps are idempotent and safe to
  retry independently; the order-service-configured check runs before either
  write, so a misconfigured deployment fails closed without leaving the
  catalog cancelled and orphaned. Reject `POST /reservations` for any event
  whose status is not `PUBLISHED` (a fast-path, non-authoritative check via
  the lean `GetEventStatus` query, distinguishing a genuinely missing event
  with a `404` from a non-bookable one with `409`); the durable, race-free
  guarantee against a reservation landing after cancellation is enforced
  inside orderservice's `CreateOrder` transaction (see below), not by this
  pre-check alone.

`orderservice` responsibilities:

- Validate idempotency headers, reservation tokens, and order command payloads.
- Handle explicit user actions via `POST /reservations/{id}/confirm` and
  `POST /reservations/{id}/cancel`.
- Handle organizer-initiated bulk cancellation via `POST /events/{id}/cancel`,
  which cancels every outstanding order (`PENDING`, `HELD`, or `CONFIRMED`) for
  the event. Unlike the single-order cancel, `CONFIRMED` orders are eligible
  here since the event itself is being called off. All matching orders
  transition together in one batched `UPDATE ... RETURNING` inside a single
  transaction (not one transaction per order — this trades per-order failure
  isolation for avoiding an O(orders) sequence of round trips on a large
  event), followed by one outbox row per cancelled order and a single
  `EventCancelled` outbox row, all in that same transaction. The endpoint is
  idempotent and safe to retry.
- `CreateOrder` closes the booking-race window against a concurrent
  `CancelEvent`: it re-checks `catalog.events.status` with `SELECT ... FOR
  SHARE` inside its own transaction. Row-level locking applies regardless of
  isolation level, so this blocks against (and always observes the outcome
  of) `apigateway`'s non-transactional `CancelEvent` update, rather than
  relying on serializable-snapshot conflict detection — which would not fire
  here, since only one side of that race is a serializable transaction.
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

- Consume `OrderCreated`, `OrderConfirmed`, `OrderCancelled`, and
  `EventCancelled` events, and run a periodic sweep for its own hold-expiry
  timeouts.
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
SeatReservationCancelled
SeatTicketTransferred
SeatTicketUsed
SeatTicketUpgraded
```

`SeatReservationCancelled` is a terminal transition, distinct from
`SeatReservationExpired`: the seat does not become available again, it
becomes permanently unbookable because its event was cancelled. It fires from
either `Held` or `Sold` (unlike expiry, which only fires from `Held`), guarded
by the same compare-and-append discipline as every other stream mutation.

## Storage Profiles

- `seatservice`: append-only event store, backed by PostgreSQL event table or
  RocksDB segments with durable WAL.
- `orderservice`: PostgreSQL tables for orders and `outbox_events`.
- Read model: PostgreSQL `projection` schema for discovery, wallet, order, and
  seat snapshots.
- Redis: idempotency keys, token buckets, hot layout locks, and short-lived
  fanout coordination.

## Event Sourcing and CQRS Mutation

`seatservice` mutations append events only:

```text
OrderCreated     -> SeatReservationHeld
OrderConfirmed   -> SeatReservationConfirmed
OrderCancelled   -> SeatReservationExpired
EventCancelled   -> SeatReservationCancelled (fanned out to every stream for
                    the event still Held or Sold)
hold expires (seatservice sweep, no order trigger) -> SeatReservationExpired
```

`viewservice` workers consume Kafka and flatten immutable facts into read
documents:

```text
inventory.events.v1 -> seat_snapshot[event_id, seat_id]              (status incl. CANCELLED)
inventory.events.v1 -> wallet_ticket[ticket_id]                      (status incl. CANCELLED)
order.events.v1     -> order_summary[user_id, order_id]
```

A `SeatReservationCancelled` event upserts `seat_snapshot.status = CANCELLED`
and, if a wallet ticket had already been issued for that seat, flips
`wallet_ticket.status` from `ISSUED` to `CANCELLED` as well.

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
7. `viewservice` updates seat as HELD and SSE broadcasts
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

Event cancellation path (organizer action, whole event called off):

```text
1. Organizer POST /organizer/events/{event_id}/cancel
2. `apigateway` verifies venue/event ownership and that orderservice is
   configured (fails closed with no writes if not), marks the event CANCELLED
   in catalog.events, and calls orderservice's POST /events/{event_id}/cancel
3. `orderservice` finds every order touching event_id and, in one batched
   transaction, marks each still PENDING/HELD/CONFIRMED order CANCELLED and
   writes an outbox OrderCancelled (reason=EVENT_CANCELLED) per order -
   unlike a user-initiated cancel, CONFIRMED orders are eligible here
4. `orderservice` writes one additional outbox EventCancelled(event_id) in
   that same transaction
5. Outbox relay publishes all of the above to Kafka
6. `viewservice` applies each OrderCancelled into order_summary as usual
7. `seatservice` consumes EventCancelled and appends SeatReservationCancelled
   to every seat stream for event_id not already Cancelled - including seats
   that were never held or sold, so no seat is left looking bookable after
   its event is cancelled. Streams are processed in bounded batches (guarded
   by compare-and-append, same as every other stream mutation) rather than
   locking every stream for the event at once, so a large venue's
   cancellation doesn't hold row locks across the whole seat map for the
   duration of the sweep.
8. `viewservice` marks each such seat CANCELLED and flips any already-issued
   wallet ticket for that seat from ISSUED to CANCELLED
9. `apigateway` rejects any new POST /reservations against a non-PUBLISHED
   event as a fast-path check, but the durable guarantee against a booking
   landing after cancellation is `orderservice`'s own `SELECT ... FOR SHARE`
   row lock on catalog.events inside CreateOrder's transaction (see the Go
   Services section above) - this blocks against and always observes
   apigateway's catalog update regardless of which side commits first, so an
   order can never be created after cancellation and then missed by step 3's
   sweep.
```

Both apigateway's catalog update and orderservice's bulk cancel are
idempotent, so a client retry after a partial failure (e.g. the network call
between them failing) is always safe to repeat in full.

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

Self-correction path (confirm loses the race against an in-flight expiry, or
against an event cancellation): if the expiry sweep times out a hold in
seatservice's authoritative event store just before orderservice consumes the
resulting SeatReservationExpired, a confirm can still succeed against
orderservice's stale local mirror. The same race can happen against an
organizer's event cancellation instead of an expiry. `seatservice`'s
compare-and-append guard then finds the stream already Expired or Cancelled
and, instead of appending SeatReservationConfirmed, publishes
SeatReservationConfirmationFailed carrying which of the two happened;
`orderservice` consumes it and corrects the order from CONFIRMED to either
EXPIRED (writing the same outbox OrderExpired the timeout path above uses) or
CANCELLED (writing outbox OrderCancelled, reason=EVENT_CANCELLED) to match
the actual cause, guarded to only ever apply to that exact CONFIRMED state.

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
