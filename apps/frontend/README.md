# Velox Frontend

SvelteKit SSR scaffold for buyer discovery, real-time seat selection, checkout, wallet, and vendor operations.

## Commands

```bash
npm run dev
npm run check
npm run build
```

Dependencies are declared in `package.json`; this scaffold intentionally does not vendor `node_modules`.

## Gateway Configuration

The UI will fetch from the internal gateway. When the gateway is unavailable, route loads fall back to local mock projection data so the UI remains navigable during frontend development.

Implemented public gateway client calls:

- `GET /events`
- `GET /events/:event_id/sections/:section_id/seats`
- `GET /events/:event_id/stream`
- `POST /reservations`
- `POST /reservations/:reservation_id/confirm`
- `GET /orders`
