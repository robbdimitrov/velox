# Security

## Auth And Sessions

Gateway sessions are HMAC-signed opaque tokens stored in the `velox_session`
cookie. Cookies are `HttpOnly`, `SameSite=Lax`, path `/`, and expire after 12
hours. Token verification checks signature, expiry, issuer, audience, and
subject before loading the user from the store.

The cookie does not set `Secure`, for the same reason headers omit HSTS: the
documented runtime is plain HTTP over `kubectl port-forward` with no TLS
termination anywhere in the deployment. Enabling TLS termination must add
`Secure` to `setSessionCookie`/`clearSessionCookie` at the same time.

Registration requires passwords from 8 to 128 bytes. Store-backed registration
hashes passwords with Argon2id. Demo in-memory mode still accepts seeded
plaintext passwords and is not a production security boundary.

Organizer routes require the `organizer` role and then an ownership check next
to the protected venue or event operation. Reserver order and wallet reads are
filtered by the authenticated user ID; client-supplied user IDs are ignored on
public routes.

## Request And Token Handling

Gateway request bodies are capped at 1 MiB. Reservation, organizer event,
organizer venue, and announcement commands reject unknown JSON fields.

Reservation creation requires `Idempotency-Key`. Confirm and cancel verify
ownership and require both `Idempotency-Key` and a signed reservation token.
Raw reservation and wallet tokens are not logged.

QR wallet tokens are short-lived HMAC tokens. Claims include ticket ID, user
ID, event ID, purpose, and expiry.

## Kafka Event Signing

Two independent HMAC-SHA256 trust boundaries protect Kafka events; compromise
of one signing key cannot forge events on the other topic.

`inventory.events.v1` (seatservice -> viewservice and orderservice,
`EVENT_SIGNING_KEY`): seatservice signs every
`SeatInventoryEvent`/`SeatReservationFailedEvent` over
`event_type|aggregate_id|aggregate_version|payload`, where `payload` is the
event's domain fields, hex-encoded into `signature`, and also carried verbatim
as `signed_payload` so a consumer can verify without reconstructing JSON. Both
viewservice and orderservice verify the signature and cross-check
`signed_payload`'s embedded order/seat identity against the event's own
`seat`/`correlation_id` fields before any state change; a missing, tampered,
or field-mismatched signature is logged and the record is dropped without
mutating state. `SeatReservationCancelled`'s `correlation_id` is the catalog
event being cancelled, not the seat's owning order, so consumers skip the
order-identity check for that event type only; orderservice does not act on
`SeatReservationCancelled` at all, so it never reaches that check.

`order.events.v1` (orderservice -> seatservice, `ORDER_EVENT_SIGNING_KEY`):
orderservice signs every `OrderCreated`/`OrderConfirmed`/`OrderCancelled`/
`OrderExpired`/`EventCancelled` envelope over `event_type|order_payload`,
where `order_payload` is the exact bytes embedded verbatim under the
envelope's `Order` key so the signed and transmitted bytes are identical.
seatservice verifies the signature against those same raw bytes before any
inventory mutation; a missing or invalid signature is logged and the record
is routed to `dlq.order.events.v1` without mutating inventory state.

`EVENT_SIGNING_KEY` and `ORDER_EVENT_SIGNING_KEY` are distinct secrets scoped
to their own trust boundary: seatservice, viewservice, and orderservice share
`EVENT_SIGNING_KEY`; orderservice and seatservice share
`ORDER_EVENT_SIGNING_KEY`. Neither key is reused across the other boundary.

## Backend Headers

`apigateway` emits only JSON or SSE. `securityHeadersMiddleware` sets these
headers on every response:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `X-XSS-Protection: 0`
- `Referrer-Policy: no-referrer`
- `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'`

The backend does not set `Strict-Transport-Security` or
`upgrade-insecure-requests` because the documented local runtime uses plain HTTP
over `kubectl port-forward`.

## Frontend Headers

`apps/frontend/src/hooks.server.ts` sets these headers on every response:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Cross-Origin-Opener-Policy: same-origin`
- `Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=(), usb=(), bluetooth=()`

`apps/frontend/svelte.config.js` uses SvelteKit nonce CSP:

```text
default-src 'self'; script-src 'self' <per-request nonce>; style-src 'self';
style-src-attr 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; font-src 'self';
object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; frame-src 'none'
```

Directive rationale:

- Remote fonts are disallowed. The frontend uses system fonts only.
- `style-src-attr 'unsafe-inline'` is limited to inline style attributes used
  for live seat-map sizing, cursor state, and health-panel gauges.
- `img-src 'self' data:` covers framework assets and DaisyUI's inline data-URI
  spinners and panel textures; event screens are otherwise image-less. Data
  URIs cannot execute script or make network requests, so this does not weaken
  the policy the way an external host allowance would.
- `connect-src 'self'` is enough because browser gateway traffic uses the
  same-origin `/api` proxy; the proxy calls `GATEWAY_URL` server-side.
- Framing is denied because reservation actions are clickjacking-sensitive and
  the app does not use iframes.

The frontend also omits `Strict-Transport-Security` and
`upgrade-insecure-requests` for the same no-TLS local-runtime reason.

## Out of Scope

CSP violation reporting is not configured because this local/portfolio runtime
has no monitoring sink for reports.

## Logging And Secrets

Logs should contain request IDs, route names, status codes, and operational
metadata. Do not log passwords, session cookies, reservation tokens, QR tokens,
request bodies, raw user-controlled text, or secret values.

Deployment secrets are generated by `scripts/deploy.sh` for local use and are
referenced by manifests rather than committed.
