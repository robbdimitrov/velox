# Security

## HTTP Security Headers (backend)

`apigateway` is JSON- and SSE-only (`writeJSON` always sets
`Content-Type: application/json`; the two stream handlers set
`text/event-stream`); it never renders HTML. `securityHeadersMiddleware` in
`apps/apigateway/api/router.go` sets, on every response regardless of route or
outcome:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY` (nothing ever has a legitimate reason to frame a
  JSON/SSE API response)
- `X-XSS-Protection: 0` (legacy auditor disabled; not relevant to non-HTML
  responses but kept for consistency with the frontend baseline)
- `Referrer-Policy: no-referrer`
- `Content-Security-Policy: default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'`
  — deliberately tighter than a browser-app CSP baseline: since this origin
  never serves a document for a browser to render, there is no legitimate
  script, style, image, font, or connect source to allow.

No `Strict-Transport-Security` and no `upgrade-insecure-requests`: this
deployment has no TLS termination (see `docs/deployment.md`; local access is
via `kubectl port-forward` over plain HTTP), so sending either header would
promise a guarantee the transport doesn't hold.

## HTTP Security Headers (frontend)

`apps/frontend/src/hooks.server.ts` sets, on every response:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: strict-origin-when-cross-origin`
- `Cross-Origin-Opener-Policy: same-origin`
- `Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=(), usb=(), bluetooth=()`
  (the app uses none of these; the "location picker" in the discovery UI is a
  city filter, not the Geolocation API, and checkout never uses the Payment
  Request API)

`apps/frontend/svelte.config.js` configures SvelteKit's nonce-based CSP
(`kit.csp.mode: 'nonce'`), which SvelteKit injects into every rendered-page
response automatically:

```
default-src 'self'; script-src 'self' <per-request nonce>; style-src 'self' https://fonts.googleapis.com;
style-src-attr 'unsafe-inline'; img-src 'self'; connect-src 'self'; font-src 'self' https://fonts.gstatic.com;
object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; frame-src 'none'
```

Rationale for directives that diverge from a generic baseline:

- `style-src`/`font-src` allow Google Fonts (`fonts.googleapis.com`,
  `fonts.gstatic.com`): `app.css` loads Space Grotesk/Space Mono via
  `@import url('https://fonts.googleapis.com/...')`, and that stylesheet in
  turn references font files on `fonts.gstatic.com`. Fonts are not
  self-hosted; self-hosting them would let both directives drop to `'self'`
  but is out of scope for a headers/CSP change.
- `style-src-attr: 'unsafe-inline'`: `SeatCanvas.svelte` (the primary
  high-density seat-map renderer) and `SystemHealthPanel.svelte` compute
  per-render pixel sizes, cursor state, and gauge widths as inline `style="..."`
  attributes from live state. That content is genuinely dynamic — it can't be
  pinned with a CSP hash or covered by SvelteKit's automatic script nonce
  (which only threads through `<script>` tags). The exception is scoped to
  `style-src-attr` only, so it permits inline `style="..."` attribute values
  but not `<style>` blocks or external stylesheets — `style-src` itself stays
  at `'self'` plus the Google Fonts origin. SvelteKit's own fixed-content
  `#svelte-announcer` route-announcer div (a static inline style, independently
  verified by hashing the exact string SvelteKit renders) is also covered by
  this, so no separate hash exception is needed here (unlike a stricter sibling
  app that has no dynamic-style requirement and can stay hash-only).
- `img-src 'self'` with no `data:`/`blob:`: event card images are a fixed set
  of bundled local SVGs (`apps/frontend/static/*.svg`); nothing in the app
  renders user-supplied or data-URI images.
- `connect-src 'self'`: every gateway call — reservation mutations, seat
  snapshot reads, and all three SSE streams (discovery ticker, seat-map
  deltas, organizer live metrics) — goes through the same-origin proxy route
  `apps/frontend/src/routes/api/[...path]/+server.ts`, which forwards
  server-to-server to `GATEWAY_URL`. The organizer metrics stream previously
  built an absolute cross-origin URL from the `PUBLIC_GATEWAY_BASE_URL` public
  env var instead of using the `/api` proxy like every other call site; since
  `apigateway` sets no CORS headers and `scripts/deploy.sh` never
  port-forwards it, that connection could not actually succeed browser-side
  under the documented local dev flow. It's fixed to use the same `/api`
  proxy pattern as the rest of the app (see `apps/frontend/src/routes/organizer/+page.ts`),
  which also means `connect-src` never needs a second, deployment-specific
  origin.
- `frame-ancestors 'none'` / `frame-src 'none'` / `X-Frame-Options: DENY`:
  nothing in the app renders or expects to be rendered inside an `<iframe>`,
  and reservation/checkout actions are clickjacking-sensitive, so framing is
  denied outright rather than scoped to `'self'`.

No `Strict-Transport-Security` and no `upgrade-insecure-requests`, for the
same no-TLS reason as the backend.

## Out of Scope

CSP violation reporting (`report-to`/`report-uri`) is intentionally not
configured. This is an unmonitored local/portfolio deployment with nowhere to
send or act on reports.
