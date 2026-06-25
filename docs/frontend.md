# Frontend

## Design Direction

Velox uses brutalist-clean minimalism for live entertainment under extreme demand. The base surface is deep carbon `#0F0F11`; interface structure uses thin slate lines, square geometry, dense grids, and high-contrast typography. State color must be functional, not decorative:

- `#FF3366`: urgency, held seats, destructive warnings, checkout expiry.
- `#5533FF`: primary action, selected seats, active filters.
- `#D7D7DE`: readable foreground text.
- `#2A2A31`: inactive borders, map section outlines, disabled states.

Use sharp 0 to 4 px radii for core controls. Prefer monospace numerals for timers, counters, and seat identifiers. Do not use soft gradients or decorative card stacks for operational flows.

## Landing and Event Discovery Page

Purpose: handle high-volume reads without touching write databases.

Layout:

1. Top command bar: logo, location picker, search input, account entry, and rate-limit friendly category tabs.
2. Live ticker strip: SSE-fed upcoming sale and sell-through messages, capped to fixed-height rows to prevent layout shift.
3. Filter rail: event type, date window, city, price band, and availability threshold. Each filter mutates URL query state and triggers debounced read-model queries.
4. Trending demand list: dense event rows with event image, venue, sale start, remaining inventory bucket, and demand score.
5. Featured grid: cached event cards for top categories, refreshed from Elasticsearch or MongoDB projections.

Implementation rules:

- Query only read models. Never call `orderservice` or `inventoryservice` from discovery.
- Cache hot discovery responses at the CDN for 1 second with stale-while-revalidate.
- `EventCard.svelte` derives scarcity badges from streamed read data using Svelte 5 Runes.
- `LiveTicker.svelte` consumes SSE and appends bounded messages into an in-memory ring buffer.

## Real-Time Stadium Seat Selector

Purpose: present accurate seat state while avoiding component-wide re-rendering.

Layout:

1. Left toolbar: section selector, zoom controls, price class toggles, accessibility filter.
2. Main viewport: Canvas for high-density seat nodes; SVG overlay for section outlines and labels.
3. Right panel: selected seats, expiration clocks, price breakdown, reserve action.
4. Bottom event log: compact live updates for section-level inventory movement.

Seat states:

- Available: grey node.
- Selected by current user: indigo node with outline.
- Held by another user: flashing crimson node until expiry.
- Sold: solid carbon node, non-interactive.
- Unknown or stale: outlined node with disabled click target.

State sync:

- WebSocket messages carry `seat_id`, `status`, `version`, `event_id`, and `expires_at_server_ms`.
- Client state is a typed seat array keyed by numeric seat index. Updates replace only changed entries.
- Reject any message with a version lower than the locally observed version for the same seat.

## High-Velocity Checkout Pipeline

Purpose: convert a held reservation into a paid ticket before the reservation lock expires.

Layout:

1. Left panel: event name, venue, selected seats, fees, total, reservation version.
2. Timer band: server-authoritative countdown displayed as `MM:SS` with monospace numerals.
3. Right panel: minimal payment form, billing summary, terms confirmation, submit button.
4. Failure strip: single-line error states for card rejection, expired hold, duplicate request, or stale reservation.

Rules:

- Svelte generates a UUID `idempotency_key` before submitting payment.
- Disable submit after the first click and attach `Idempotency-Key` plus reservation token headers.
- Timer display is client-side interpolation from `expires_at_server_ms` and server time offset, but backend expiry is authoritative.
- On `SeatReservationExpired`, immediately clear checkout state and return the user to the seat map.

## Secure Ticket Wallet and History Ledger

Purpose: expose the ticket lifecycle from event-sourced history.

Layout:

1. Wallet header: upcoming tickets, transfer status, identity verification state.
2. Ticket pass list: QR code, event, seat, gate, transfer controls.
3. Provenance ledger: expandable immutable timeline per ticket.
4. History filters: Issued, Transferred, Used, Upgraded, Refunded.

Ledger examples:

```text
2026-06-24T22:30:00Z TicketIssued ticket_id=velox_8831
2026-06-24T22:32:11Z PaymentConfirmed provider=stripe charge_id=...
2026-06-24T22:40:19Z TicketUpgraded tier=VIP_Lounge
```

Rules:

- Wallet reads from projections, not the event store directly.
- QR payloads must be short-lived signed tokens, never raw ticket IDs alone.
- Ledger rows must include event type, timestamp, actor, and correlation ID.
