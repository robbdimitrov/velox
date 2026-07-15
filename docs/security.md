# Security

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
default-src 'self'; script-src 'self' <per-request nonce>; style-src 'self' https://fonts.googleapis.com;
style-src-attr 'unsafe-inline'; img-src 'self'; connect-src 'self'; font-src 'self' https://fonts.gstatic.com;
object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'; frame-src 'none'
```

Directive rationale:

- Google Fonts stays allowed because `app.css` imports fonts from
  `fonts.googleapis.com` and `fonts.gstatic.com`.
- `style-src-attr 'unsafe-inline'` is limited to inline style attributes used
  for live seat-map sizing, cursor state, and health-panel gauges.
- `img-src 'self'` is enough because event images are bundled local SVGs.
- `connect-src 'self'` is enough because browser gateway traffic uses the
  same-origin `/api` proxy; the proxy calls `GATEWAY_URL` server-side.
- Framing is denied because checkout and reservation actions are
  clickjacking-sensitive and the app does not use iframes.

The frontend also omits `Strict-Transport-Security` and
`upgrade-insecure-requests` for the same no-TLS local-runtime reason.

## Out of Scope

CSP violation reporting is not configured because this local/portfolio runtime
has no monitoring sink for reports.
