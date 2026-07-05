# Frontend

## Stack

| Item       | Value                   |
| ---------- | ----------------------- |
| Framework  | SvelteKit SSR, Svelte 5 |
| Language   | TypeScript              |
| Styling    | Tailwind                |
| Components | DaisyUI                 |
| Icons      | Lucide                  |

## Design Direction

Velox uses a "Moon Landing" / aerospace aesthetic to handle live entertainment under extreme demand. The base surface is deep space carbon `#050505`; interface structure uses lunar silver lines, square geometry, dense grids, and high-contrast typography. State color must be functional, not decorative:

- `#FACC15` (Apollo Visor Gold): primary action, selected seats, active filters, glowing stage.
- `#EF4444` (Control Panel Red): urgency, held seats by others, destructive warnings, checkout expiry.
- `#F8FAFC` (Spacesuit White): readable foreground text.
- `#1F2937` (Dusty Gray): inactive borders, map section outlines, disabled states.
- `#94A3B8` (Lunar Silver): available seats, secondary elements.

Use sharp 0 to 4 px radii for core controls. Prefer `Space Grotesk` for UI text and `Space Mono` numerals for timers, counters, and seat identifiers. The interface should feel like a classic, high-reliability aerospace control center.

## Landing and Event Discovery Page

Purpose: handle high-volume reads without touching write databases.

Layout:

1. Top command bar: logo, location picker, search input, account entry, and
   rate-limit friendly category tabs.
2. Live ticker strip: SSE-fed upcoming sale and sell-through messages, capped to
   fixed-height rows to prevent layout shift.
3. Filter rail: event type, date window, city, price band, and availability
   threshold. Each filter mutates URL query state and triggers debounced
   read-model queries.
4. Trending demand list: dense event rows with event image, venue, sale start,
   remaining inventory bucket, and demand score.
5. Featured grid: cached event cards for top categories, refreshed from
   Elasticsearch or MongoDB projections.

Implementation rules:

- Query only read models. Never call `orderservice` or `seatservice` from
  discovery.
- Cache hot discovery responses at the CDN for 1 second with
  stale-while-revalidate.
- `EventCard.svelte` derives scarcity badges from streamed read data using
  Svelte 5 Runes.
- `LiveTicker.svelte` consumes SSE and appends bounded messages into an
  in-memory ring buffer.

## Real-Time Stadium Seat Selector

Purpose: present accurate seat state while avoiding component-wide re-rendering.

Layout:

1. Left toolbar: section selector, zoom controls, price class toggles,
   accessibility filter.
2. Main viewport: Canvas for high-density seat nodes; SVG overlay for section
   outlines and labels.
3. Right panel: selected seats, expiration clocks, price breakdown, reserve
   action.
4. Bottom event log: compact live updates for section-level inventory movement.

Seat states:

- Available: grey node.
- Selected by current user: indigo node with outline.
- Held by another user: flashing crimson node until expiry.
- Sold: solid carbon node, non-interactive.
- Unknown or stale: outlined node with disabled click target.

State sync:

- WebSocket messages carry `seat_id`, `status`, `version`, `event_id`, and
  `expires_at_server_ms`.
- Client state is a typed seat array keyed by numeric seat index. Updates
  replace only changed entries.
- Reject any message with a version lower than the locally observed version for
  the same seat.
- Live SSE and WebSocket effects must close streams and fallback timers on
  component teardown. Malformed live payloads are ignored or logged in bounded
  UI state rather than breaking the seat selector.

## High-Velocity Reservation Pipeline

Purpose: convert a held reservation into a confirmed ticket before the reservation
lock expires.

Layout:

1. Left panel: event name, venue, selected seats, reservation
   version.
2. Timer band: server-authoritative countdown displayed as `MM:SS` with
   monospace numerals.
3. Right panel: reservation confirmation prompt, terms of service acceptance,
   submit button.
4. Failure strip: single-line error states for expired hold,
   duplicate request, or stale reservation.

Rules:

- Svelte generates a UUID `idempotency_key` before submitting a reservation
  confirm/cancel action.
- Disable submit after the first click and attach `Idempotency-Key` plus
  reservation token headers.
- Timer display is client-side interpolation from `expires_at_server_ms` and
  server time offset, but backend expiry is authoritative.
- On `SeatReservationExpired`, immediately clear checkout state and return the
  user to the seat map.

## Secure Ticket Wallet and History Ledger

Purpose: expose the ticket lifecycle from event-sourced history.

Layout:

1. Wallet header: upcoming tickets, transfer status, identity verification
   state.
2. Ticket pass list: QR code, event, seat, gate, transfer controls.
3. Provenance ledger: expandable immutable timeline per ticket.
4. History filters: Issued, Transferred, Used, Upgraded.

Ledger examples:

```text
2026-06-24T22:30:00Z TicketIssued ticket_id=velox_8831
2026-06-24T22:32:11Z SeatReservationConfirmed order_id=ord_5521
2026-06-24T22:40:19Z TicketUpgraded tier=VIP_Lounge
```

Rules:

- Wallet reads from projections, not the event store directly.
- QR payloads must be short-lived signed tokens, never raw ticket IDs alone.
- Ledger rows must include event type, timestamp, actor, and correlation ID.
