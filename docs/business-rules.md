# Business Rules

This document defines behavior users can rely on. When current code is short of
the target, the gap is called out explicitly.

## Identity And Sessions

- Users register with email, password, and role.
- Passwords must be 8 to 128 bytes.
- Empty role means `reserver`; valid explicit roles are `reserver` and
  `organizer`.
- Store-backed passwords are Argon2id hashes.
- Session cookies are `HttpOnly`, `SameSite=Lax`, path `/`, and expire after
  12 hours.
- Session tokens are HMAC-signed and validate issuer, audience, expiry, and
  subject.
- Login failures are counted per email and client IP. Five failures lock login
  for 5 minutes.

## Roles And Ownership

- Buyers can browse, reserve, confirm/cancel their own reservations, read their
  own orders, and read their own wallet.
- Organizers can use organizer routes only after role check.
- Organizer event actions require ownership through the event venue.
- Venue ownership is represented by `catalog.user_venues`.
- Staff listing reads `catalog.user_venues`. Staff assignment is not
  implemented, so the showcase UI exposes read-only venue access and no active
  invite or assignment controls.

## Venue And Event Rules

- A venue has name, city, address, and capacity.
- Venue creation accepts optional grid section templates. Each template defines
  section ID, name, row count, seats per row, default price, and whether edge
  seats are accessible.
- If no template is supplied, store-backed venue creation creates a default A/B
  template: rows A-D, seats 01-10, geometry coordinates, accessibility flags on
  edge seats, and a default price.
- Venue names are required and capped at 120 characters. City is capped at 80,
  address at 240, capacity at 250000, and section templates are capped at 8
  sections, 26 rows, and 50 seats per row.
- Organizer event create requires an owned venue in store-backed mode.
- Event create accepts `venue_id`, `name`, `description`, `category`,
  `starts_at`, `sale_starts_at`, `image_key`, and optional `timezone`.
- Event names are required and capped at 120 characters. Descriptions are capped
  at 5000 characters.
- `starts_at` must be after `sale_starts_at`.
- Categories are `Concerts`, `Sports`, `Theatre`, and `Festivals`.
- Image keys are `event-midnight-array`, `event-final-whistle`, and
  `event-zero-hour`.
- Events are bookable only when `catalog.events.status = PUBLISHED`.
- `CANCELLED` events are not bookable.
- Draft and completed statuses are target states, not implemented states.

## Reservation Rules

- A reservation create command requires auth, `Idempotency-Key`, `event_id`,
  `section_id`, and 1 to 8 `seat_ids`.
- The gateway performs an early projection check for event status and seat
  availability.
- Seatservice remains the authoritative double-allocation guard through
  expected-version event stream appends.
- Orderservice re-checks event status inside `CreateOrder` using a row lock, so
  a reservation cannot commit after event cancellation.
- Idempotency keys are scoped by service, user, and key. Same body returns the
  same response reference; different body returns conflict.
- Current create-order idempotency TTL is 24 hours.
- Reservation create returns a gateway-signed reservation token.
- Confirm and cancel require `Reservation-Token` and `Idempotency-Key`.
- Reservation-token claims bind reservation ID, order ID, user ID, event ID,
  section ID, seat IDs, issued timestamp, and expiry timestamp. Confirm and
  cancel reject missing, invalid, cross-user, or expired tokens.
- Confirm and cancel idempotency keys are scoped by action, user, and key. Same
  action data replays the same gateway response; different token/action data
  returns conflict.

## Hold Deadlines

- Seatservice computes the 10-minute hold deadline and persists it in
  `SeatReservationHeld`.
- Gateway reservation responses and signed reservation tokens use the same
  10-minute duration for the client countdown and command-token expiry.
- Clients display countdowns from server timestamps but cannot extend holds.
- Expiry appends `SeatReservationExpired` only if the latest seat event is
  still held and current time is past the persisted deadline.
- Confirmation after expiry must be rejected or corrected by seatservice if a
  stale order mirror briefly accepted it.

## Checkout Decision

Velox currently uses reservation-only checkout. The user confirms a held
reservation; no real payment processor or payment form is part of the product.

If a local payment simulator is added later, it must persist simulated payment
state and use idempotent confirmation. Until then UI copy should use
"reservation" or "confirm reservation", not "charge" or "payment".

## Order Lifecycle

Order states:

- `PENDING`: order row created; seatservice has not yet confirmed the hold.
- `HELD`: inventory hold succeeded.
- `CONFIRMED`: buyer confirmed a held reservation.
- `CANCELLED`: buyer cancellation or event cancellation.
- `FAILED`: seat hold failed.
- `EXPIRED`: hold expired before confirmation.

User cancellation is valid from `PENDING` or `HELD`. Event cancellation can also
cancel `CONFIRMED` orders because the event itself is no longer valid.

## Seat Contention

- Seat stream key is `seat:{event_id}:{section_id}:{seat_id}`.
- Every inventory mutation appends with expected version.
- Version mismatch prevents double allocation.
- Projection reads may be stale and never authorize booking by themselves.
- SSE deltas must be applied only when the incoming version is newer than the
  local version.

## Event Cancellation

- Organizer cancel verifies event ownership first.
- Gateway fails closed if orderservice is not configured.
- Catalog status changes to `CANCELLED`.
- Orderservice bulk-cancels outstanding orders in one transaction and writes
  one outbox row per cancelled order plus one `EventCancelled` row.
- Seatservice marks held, sold, and never-touched seats as terminal cancelled
  so no seat appears bookable after cancellation.
- Wallet tickets for cancelled events become `CANCELLED`.

## Wallet And QR Tokens

- Wallet reads projection tickets only.
- QR tokens are short-lived HMAC tokens minted by the gateway.
- QR token TTL is currently 90 seconds. Claims include ticket ID, user ID,
  event ID, purpose, and expiry.
- Ticket ledger rows come from inventory events and include event type,
  timestamp, actor, and correlation ID.
- Transfer/use/upgrade actions are not implemented; active controls must be
  absent or disabled.

## Projection Lag

- Read APIs expose `projection_lag_ms` or `snapshot_age_ms` where implemented.
- Commands return authoritative results from write services.
- If lag exceeds a configured threshold, risky UI actions should freeze or
  clearly show stale state.
- Confirm responses return backend-owned wallet ticket IDs visible in the
  projection at response time.
- Wallet issuance must tolerate duplicate and out-of-order events.

Current wallet gap: there is no durable pending-ticket buffer or repair worker
for inventory confirmation arriving before order summary.

## Rate Limits And Bounds

- Gateway reservation create/confirm/cancel routes use token-bucket rate
  limiting when Redis is configured.
- Reservation request bodies are capped to 1 MiB and 8 seats.
- Announcement title is capped at 200 characters.
- Announcement body is capped at 5000 characters.
- Announcement feed reads return at most 100 rows.

Future pagination must define default and maximum limits before broad
collections grow beyond showcase size.
