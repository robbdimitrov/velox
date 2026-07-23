# API Contract

Velox exposes one browser-facing JSON/SSE contract through `apigateway`.
SvelteKit proxies browser calls from `/api/*` to the same gateway path without
the `/api` prefix. Gateway-native routes below are canonical unless explicitly
marked as a compatibility alias.

## Common Rules

- All non-SSE responses are JSON.
- Error responses use `{"error":"stable_code"}`.
- Gateway request bodies are capped at 1 MiB by `limitBody`.
- Command endpoints require a session cookie unless noted otherwise.
- Organizer endpoints require role `organizer` and then venue or event
  ownership.
- Discovery and announcement reads are cacheable. Per-user and command
  endpoints are not shared-cacheable.
- Collection pagination is not implemented yet. Until it is added, responses
  are bounded by handler-specific caps where present.

Current JSON unknown-field policy:

| Endpoint family | Policy |
| --- | --- |
| `POST /reservations` | Strict; unknown fields and trailing JSON rejected. |
| `POST /reservations/{reservationId}/confirm` | Header-only command; body ignored today. |
| `POST /reservations/{reservationId}/cancel` | Header-only command; body ignored today. |
| `POST /organizer/events/{eventId}/announcements` | Strict. |
| `POST /organizer/events` | Strict. |
| Internal orderservice `POST /orders` | Strict. |
| Auth and venue create | Loose today; hardening should replace broad DTOs with request DTOs. |

## Public Routes

### `GET /healthz`

Returns process liveness.

```json
{"status":"ok"}
```

### `GET /readyz`

Checks required gateway dependencies. Store-backed mode pings PostgreSQL with a
2 second timeout.

- `200 {"status":"ok"}`
- `503 {"status":"degraded"}` when a hard dependency is unavailable

### `GET /events`

Public discovery read. Query parameters:

| Name | Rule |
| --- | --- |
| `q` | Trimmed and lowercased; capped to 120 characters. |
| `city` | Exact case-insensitive city match; `all` disables the filter. |
| `available` | Defaults to available-only. `false` includes fully reserved events. |
| `date` | `Today`, `This week`, `This month`, or any other value for no filter. |

Response:

```json
{
  "events": [
    {
      "id": "evt_neon_riot",
      "venue_id": "ven_velox_arena",
      "status": "PUBLISHED",
      "name": "Neon Riot Live",
      "category": "Concerts",
      "description": "Arena-scale synth and alt-pop with synchronized fan drops.",
      "venue": "Velox Arena",
      "city": "Chicago",
      "starts_at": "2026-08-15T20:00:00Z",
      "timezone": "America/Chicago",
      "section_ids": ["A", "B"],
      "seats_total": 80,
      "seats_open": 80,
      "demand_score": 94
    }
  ],
  "projection_lag_ms": 12,
  "cache_status": "healthy"
}
```

The gateway owns event metadata. Published events are immediately reservable,
and the frontend derives remaining buckets from `seats_open`. `cache_status` is
`healthy`, `degraded` (cache backend unreachable), or `disabled` (no cache
backend configured); it reflects a live Redis ping, not the presence of
cached data for this specific request.

### `GET /events/{eventId}`

Public event detail read.

```json
{"event": {"id":"evt_neon_riot","name":"Neon Riot Live"}, "projection_lag_ms": 0}
```

Event detail is the canonical event-page source. Discovery is only a fallback
backfill if an older gateway omits a field the frontend can still derive.

### `GET /events/{eventId}/sections/{sectionId}/seats`

Public section seat snapshot read.

```json
{
  "seats": [
    {
      "event_id": "evt_neon_riot",
      "section_id": "A",
      "seat_id": "A-01",
      "row": "A",
      "number": 1,
      "x": 44,
      "y": 42,
      "accessibility": true,
      "status": "AVAILABLE",
      "version": 0,
      "expires_at_server_ms": 0
    }
  ],
  "snapshot_age_ms": 4
}
```

`projection.seat_snapshots` is the read source. Seatservice's event stream is
the write authority. Store-backed responses include backend-owned geometry and
accessibility; the frontend only derives fallback geometry for older or
in-memory responses missing those fields.

### `GET /events/{eventId}/stream`

Public SSE feed for seat updates.

Events:

| Event | Payload |
| --- | --- |
| `heartbeat` | `{"event_id":"evt_neon_riot"}` |
| `update` | Store notification payload, expected to include changed seat state. |

Current implementation is event-scoped and ignores `section_id` query filters.

### `GET /events/{eventId}/announcements`

Public announcement feed, newest first, capped at 100 rows in store-backed mode.

```json
{"announcements":[{"id":"...","event_id":"evt","title":"Doors","body":"Open","severity":"INFO","created_at":"..."}]}
```

## Auth Routes

### `POST /auth/register`

Loose JSON body today:

```json
{"email":"reserver@example.test","password":"secret","role":"reserver"}
```

`role` may be empty or `reserver`, or `organizer`. Passwords are Argon2id-hashed
in store-backed mode. Demo in-memory login also accepts seeded plaintext
passwords.

Response: `200 {"user":{"id":"...","email":"...","role":"reserver"}}` plus
`velox_session` cookie.

### `POST /auth/login`

Body:

```json
{"email":"reserver@example.test","password":"secret"}
```

Invalid attempts are rate-limited per email and client IP after 5 failures for
5 minutes. Response matches register.

### `POST /auth/logout`

Clears `velox_session`. Response is `204 No Content`.

### `GET /auth/me`

Requires cookie. Response: `{"user":...}` or `401 {"error":"authentication_required"}`.

## Reserver Routes

### `POST /reservations`

Requires auth, `Idempotency-Key`, strict JSON, and 1 to 8 seats.

Request:

```json
{"event_id":"evt_neon_riot","section_id":"A","seat_ids":["A-01","A-02"]}
```

Response:

```json
{
  "order": {
    "id": "ord_...",
    "reservation_id": "res_ord_...",
    "reservation_token": "opaque-hmac-token",
    "event_id": "evt",
    "section_id": "A",
    "seat_ids": ["A-01"],
    "seats": [{"seat_id": "A-01"}],
    "status": "PENDING",
    "expires_at_server_ms": 1760000000000,
    "server_time_ms": 1759999700000
  }
}
```

`reservation_token` is signed by the gateway and is opaque to the browser. Its
claims include reservation ID, order ID, user ID, event ID, section ID, seat
IDs, `expires_at`, `expires_at_server_ms`, `issued_at`, and
`issued_at_server_ms`. Clients use `server_time_ms` plus
`expires_at_server_ms` for countdown display.

Stable errors include `missing_idempotency_key`, `invalid_json`,
`invalid_seat_count`, `event_not_bookable`, `seat_not_available`,
`section_not_found`, `seat_not_found`, `idempotency_key_conflict`, and
`upstream_error`.

### `POST /reservations/{reservationId}/confirm`

Requires auth, `Idempotency-Key`, and `Reservation-Token`. The gateway verifies
the token signature, purpose, issuer, audience, reservation ID, order ID, user
ID, and expiry before proxying to orderservice. The gateway also verifies order
ownership by resolving `reservationId -> orderId`.
Confirm is valid only after the asynchronous inventory pipeline marks the order
`HELD`; clients should poll `GET /orders/{orderId}` or consume live state before
submitting confirm.

Response:

```json
{"order_id":"...","status":"CONFIRMED","wallet_ticket_ids":["tkt_..."]}
```

`wallet_ticket_ids` is backend-owned. It may be empty while the wallet
projection catches up; Phase 4 owns the durable out-of-order issuance repair.

Stable errors include `reservation_token_required`,
`reservation_token_invalid`, `reservation_token_expired`,
`missing_idempotency_key`, `idempotency_key_conflict`,
`order_not_confirmable`, `order_not_found`, and `upstream_error`.

### `POST /reservations/{reservationId}/cancel`

Same token, ownership, and idempotency requirements as confirm. Response:

```json
{"order_id":"...","status":"CANCELLED","wallet_ticket_ids":[]}
```

### `GET /orders`

Requires auth. Returns only the caller's orders:

```json
{"orders":[{"id":"...","status":"HELD"}]}
```

### `GET /orders/{orderId}`

Requires auth and ownership. Returns `{"order":...}` or `404`.

### `GET /wallet/tickets`

Requires auth. Store-backed mode reads projection tickets and mints short-lived
QR tokens. QR token claims include ticket ID, user ID, event ID, purpose
`qr_ticket`, and expiry.

```json
{"verification_state":"VERIFIED","tickets":[{"ticket_id":"...","status":"ISSUED","qr_token":"..."}]}
```

Viewservice buffers confirmed inventory events in
`projection.pending_wallet_ticket_events` when the matching order summary has
not arrived yet, then drains those rows when order projection catches up.

## Organizer Routes

### `GET /organizer/events`

Requires organizer role. Returns events owned by the organizer.

### `POST /organizer/events`

Requires organizer role and strict JSON. Request:

```json
{
  "id": "evt_optional",
  "venue_id": "ven_123",
  "name": "Show",
  "description": "Short public event copy.",
  "category": "Concerts",
  "starts_at": "2026-09-01T20:00:00Z",
  "timezone": "UTC"
}
```

Store-backed mode verifies the organizer owns `venue_id`, inserts
`catalog.events`, materializes `catalog.event_sections`, creates inventory
streams, and seeds projection seat snapshots from `catalog.venue_seats`.

`id`, `description`, `category`, and `timezone` may be omitted. Defaults are a
generated ID, empty description, `Concerts`, and `UTC`.

Validation:

- `name` is required and capped at 120 characters.
- `description` is capped at 5000 characters.
- `starts_at` is required and must be in the future.
- `category` must be `Concerts`, `Sports`, `Theatre`, or `Festivals`.
- missing or unowned venues return `404 {"error":"venue_not_found"}`.

Compatibility alias: `POST /api/organizer/events` remains gateway-native for
older callers. Browser code should use `/api/organizer/events`, which the
SvelteKit proxy forwards to canonical gateway `POST /organizer/events`.

### `GET /organizer/events/{eventId}/orders`

Requires event ownership. Returns `{"orders":[]}` from current in-memory order
state.

### `GET /organizer/events/{eventId}/inventory`

Requires event ownership. Returns aggregate counts:

```json
{"inventory":{"AVAILABLE":40,"HELD":2,"CONFIRMED":38},"active_holds":2}
```

### `GET /organizer/events/{eventId}/metrics/stream`

Requires event ownership. SSE stream sends metrics JSON from projections in
store-backed mode: confirmed reservation count, active holds, available seats,
section availability percentages, projection lag, and an inventory-derived
demand score. Legacy `GET /organizer/metrics/stream` remains for the current
organizer overview and chooses the first in-memory organizer event.

### `POST /organizer/events/{eventId}/announcements`

Requires event ownership and strict JSON:

```json
{"title":"Doors update","body":"Doors open on schedule.","severity":"INFO"}
```

`severity` defaults to `INFO` and must be `INFO`, `SCHEDULE_CHANGE`, or
`CANCELLATION`.

### `POST /organizer/events/{eventId}/cancel`

Requires event ownership. Fails closed with `503 order_service_unavailable` if
orderservice is not configured. On success, catalog event status becomes
`CANCELLED`, orderservice bulk-cancels outstanding orders, and the response is:

```json
{"event_id":"evt","status":"CANCELLED","cancelled_orders":3}
```

### `GET /organizer/venues`

Requires organizer role. Returns venues owned by the organizer.

Compatibility alias: `GET /api/organizer/venues`.

### `POST /organizer/venues`

Requires organizer role and strict JSON. Request:

```json
{
  "id": "ven_optional",
  "name": "Velox Arena",
  "city": "Chicago",
  "address": "100 Arena Way",
  "capacity": 10000,
  "sections": [
    {
      "section_id": "A",
      "name": "Main Floor",
      "row_count": 4,
      "seats_per_row": 10,
      "accessible_edge_seats": true
    }
  ]
}
```

Store-backed mode inserts `catalog.venues` and owner row in
`catalog.user_venues`, then creates section templates and generated venue seats
in the same transaction. If `sections` is omitted, the gateway creates the
default A/B template with 80 seats total.

Validation: name, city, address, and positive capacity are required. At most 8
sections are accepted. Each section requires a unique `section_id`, 1 to 26
rows, and 1 to 50 seats per row.

Compatibility alias: `POST /api/organizer/venues`.

### `GET /organizer/venues/{venueId}/staff`

Requires organizer role and venue ownership. Returns users attached through
`catalog.user_venues`.

Compatibility alias: `GET /api/organizer/venues/{id}/staff`.

### `POST /organizer/venues/{venueId}/staff`

Planned, not implemented. The showcase UI exposes only the read-only access
list until staff assignment persists.

## Internal Orderservice Routes

Gateway calls these over the cluster network:

- `POST /orders`
- `POST /orders/{id}/confirm`
- `POST /orders/{id}/cancel`
- `POST /events/{id}/cancel`

These are not public browser routes. They enforce their own 1 MiB body cap,
strict JSON for order creation, transactional idempotency for create, and safe
error mapping.
