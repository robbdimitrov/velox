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

Velox uses a crafted command-center aesthetic for live entertainment under
extreme demand. The base surface is deep carbon `#07080B`; interface structure
uses cool panel layers, square geometry, dense grids, and high-contrast
typography. State color must be functional, not decorative:

- `#F2B84B` (signal amber): primary action, selected seats, active filters, key operational highlights.
- `#39D6C8` (electric teal): accent detail, secondary live-state emphasis, and contrast against amber.
- `#FF5C5C` (urgency red): held seats by others, destructive warnings, checkout expiry, cancellation state.
- `#F7F1E8` (warm ink): readable foreground text.
- `#273244` (line blue): inactive borders, map section outlines, disabled states.
- `#8FA3B8` (muted slate): secondary text and supporting metadata.

Use sharp 0 to 4 px radii for core controls. Prefer `Space Grotesk` for UI text and `Space Mono` numerals for timers, counters, and seat identifiers. The interface should feel like a classic, high-reliability aerospace control center.

Compose frontend styling with Tailwind utilities and DaisyUI components only.
Do not add app-specific CSS classes or component rules to `app.css`; keep that
file limited to Tailwind/DaisyUI setup and design tokens.

The global app frame in `+layout.svelte` owns the 80rem page width, horizontal
viewport inset, top/bottom padding, and the nav-to-content `gap-6`. Routes
inside it should use `w-full`, or a narrower `max-w-*` only when the flow itself
is intentionally narrower.

Screen widths use a small set of Tailwind tiers:

- `max-w-7xl`: discovery, seat maps, wallet, organizer dashboards, and other multi-panel workspaces.
- `max-w-5xl`: two-panel confirmation and review flows.
- `max-w-3xl`: forms and single-column organizer setup flows.
- `max-w-md`: login, registration, and compact modal-like entry screens.

Use the same 6-unit gap for major panel groups. Do not split nav-to-content
spacing across header padding, wrapper padding, and page padding.

Use DaisyUI form controls and button variants first, then Tailwind utilities
when DaisyUI is not specific enough. Keep secondary and destructive actions
explicit with neutral or urgency tokens.

## Landing and Event Discovery Page

Purpose: handle high-volume reads without touching write databases.

Layout:

1. Top command bar: logo, location picker, search input, account entry, and
   rate-limit friendly category tabs.
2. Live ticker strip: SSE-fed upcoming sale and sell-through messages, capped to
   fixed-height rows to prevent layout shift.
3. Filter rail: event type, date window, city, and availability threshold. Each
   filter mutates URL query state and triggers debounced read-model queries.
4. Trending demand list: dense event rows with event image, venue, sale start,
   remaining inventory bucket, and demand score.
5. Top venue rail: venue cards derived from event projection summaries until a
   dedicated public venue read model exists.
6. Featured grid: cached event cards for top categories, refreshed from
   projection reads.

Implementation rules:

- Query only read models. Never call `orderservice` or `seatservice` from
  discovery.
- Search, city, date, and availability filters are URL-backed and issue
  debounced `GET /events` discovery reads.
- Public venue discovery must be derived from discovery projections unless the
  gateway exposes an explicit cacheable venue read endpoint.
- Cache hot discovery responses at the CDN for 1 second with
  stale-while-revalidate.
- `EventCard.svelte` derives scarcity badges from streamed read data using
  Svelte 5 Runes.
- `LiveTicker.svelte` consumes SSE and appends bounded messages into an
  in-memory ring buffer.

## Real-Time Stadium Seat Selector

Purpose: present accurate seat state while avoiding component-wide re-rendering.

Layout:

1. Left toolbar: section selector, zoom controls, seat attribute toggles, and
   accessibility filter.
2. Main viewport: Canvas for high-density seat nodes; SVG overlay for section
   outlines and labels.
3. Right panel: selected seats, expiration clocks, reservation summary, and
   reserve action.
4. Bottom event log: compact live updates for section-level inventory movement.

Seat states:

- Available: grey node.
- Selected by current user: indigo node with outline.
- Held by another user: flashing crimson node until expiry.
- Sold: solid carbon node, non-interactive.
- Unknown or stale: outlined node with disabled click target.

State sync:

- SSE messages carry `seat_id`, `status`, `version`, `event_id`, and
  `expires_at_server_ms`.
- Client state is a typed seat array keyed by numeric seat index. Updates
  replace only changed entries.
- Reject any message with a version lower than the locally observed version for
  the same seat.
- Live SSE effects must close streams and fallback timers on
  component teardown. Malformed live payloads are ignored or logged in bounded
  UI state rather than breaking the seat selector.

## Event Announcement Feed

Purpose: let an organizer post public updates (schedule changes, cancellation
notices) that anyone viewing the event page can read, without the concurrency
concerns of the seat/reservation pipeline.

Layout:

- An "Event Updates" panel on the event page, newest-first, severity-tinted
  (`INFO` neutral, `SCHEDULE_CHANGE` warning tone, `CANCELLATION` in Control
  Panel Red).
- On the organizer dashboard, a small post form (title, body, severity).

Rules:

- Public read (`GET /events/{id}/announcements`); no auth, no per-user state,
  cacheable like discovery reads.
- No live push - the announcement feed changes far less often than seat
  availability, so the event page simply re-fetches on load rather than
  holding an SSE connection open for it.
- If the event's own status is `CANCELLED`, the event page shows a persistent
  banner and disables the reserve action, regardless of what the announcement
  feed contains. Seat tiles remain visually togglable in this state (the
  banner and disabled Reserve button are the actual gate); the authoritative
  block against booking is enforced server-side by the reservation and order
  pipelines regardless of any client-side state.

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
4. History filters: Issued, Transferred, Used, Upgraded, Cancelled.

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
- A ticket whose event was cancelled shows status Cancelled and disables
  transfer, regardless of its prior transfer status.
