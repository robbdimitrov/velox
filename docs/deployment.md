# Deployment Support

Velox support artifacts are intentionally limited to contracts, logical data
schemas, and Kubernetes manifests until runtime services are scaffolded.

## Protobuf

The reservation MVP transport contract lives in `pkg/pb/velox.proto`. It covers:

- reservation creation, reservation confirmation, and order lookup RPCs;
- signed Kafka event envelopes with correlation, causation, aggregate version,
  schema version, and occurrence time metadata;
- order, payment, reservation, ticket, and seat-delta messages used by the
  order, inventory, and projection services.

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
set for local smoke testing. The current Go gateway also carries an in-memory
seed so the reservation flow can be exercised before PostgreSQL wiring is added.

## Kubernetes

Manifests live in `deploy/` and create:

- `velox` namespace;
- PostgreSQL StatefulSet and service;
- Redpanda StatefulSet and service;
- Dragonfly Deployment and service;
- service Deployments for `frontend`, `apigateway`, `orderservice`,
  `inventoryservice`, and `projectionservice`.

Secrets are referenced, not committed. Create them outside the repository before
starting pods:

`scripts/deploy.sh` creates generated development secrets when they do not
already exist. For shared or production-like clusters, create managed secrets
with the same names before running the script.

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
`scripts/deploy.sh --skip-build` when images are already available locally. Full
`kind` cluster creation and image loading are still follow-up automation work.
