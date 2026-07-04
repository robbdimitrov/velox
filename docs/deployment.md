# Deployment Support

Velox deployment artifacts now include runtime images, database initialization,
Kubernetes manifests, and a local deploy script for the reservation MVP.

## Protobuf

The reservation MVP transport contract lives in `pkg/pb/velox.proto`. It covers:

- reservation creation, reservation confirmation, and order lookup RPCs;
- signed Kafka event envelopes with correlation, causation, aggregate version,
  schema version, and occurrence time metadata;
- order, payment, reservation, ticket, and seat-delta messages used by the
  order, seat, and view services.

Generated language bindings are not committed yet.

## Database

`apps/database/migrations/001_init_logical_schemas.sql` creates three logical
PostgreSQL schemas:

- `orders`: order state, payment state, idempotency keys, and transactional
  outbox rows;
- `inventory`: seat event streams, immutable inventory events, and reservation
  expiry state;
- `projection`: processed-event dedupe, seat snapshots, order summaries, and
  wallet tickets.

`apps/database/seeds/001_demo_reservation_mvp.sql` adds a small demo event seat
set for local smoke testing. The local database image copies migrations and
seeds into Postgres' `docker-entrypoint-initdb.d` directory, so a fresh volume
is initialized on first startup.

## Kubernetes

Manifests live in `deploy/` and create:

- `velox` namespace;
- Database StatefulSet and service;
- Broker StatefulSet and service;
- Cache Deployment and service;
- service Deployments for `frontend`, `apigateway`, `orderservice`,
  `seatservice`, and `viewservice`.

Secrets are referenced, not committed. `scripts/deploy.sh` creates or updates
generated development secrets for local use. For shared or production-like
clusters, create managed secrets with the same names and values before running
the script, or re-apply them after local development runs.

## Commands

```bash
make proto-check
make db-check
make k8s-check
make deploy-dry-run
make deploy
```

The deploy script uses `kubectl` by default, builds service images through
`make`, applies manifests to the current Kubernetes context, waits for rollouts,
and starts port-forwards:

- frontend: `http://localhost:8080`
- gateway: `http://localhost:8081`

Use `scripts/deploy.sh --dry-run` for client-side manifest validation and
`scripts/deploy.sh --skip-build` when images are already available locally.
Override `KUBECTL`, `IMAGE_PREFIX`, and `GIT_SHA` to match a non-default
cluster or registry. Images are pushed to `IMAGE_PREFIX-<service>:<GIT_SHA>`
(default `localhost:5000/velox-<service>:<git-sha>`); when a `velox-control-plane`
kind node is running, `scripts/deploy.sh` also loads the built images directly
into it. Full `kind` cluster creation is still follow-up automation work.

Frontend container builds use `npm ci` against the committed lockfile in the
builder stage and copy only the adapter-node build output into the Node runner
image.
