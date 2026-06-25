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

Manifests live in `deploy/k8s/` and create:

- `velox` namespace;
- PostgreSQL StatefulSet and service;
- Redpanda StatefulSet and service;
- Dragonfly Deployment and service;
- placeholder service Deployments for `apigateway`, `orderservice`,
  `inventoryservice`, and `projectionservice`.

Secrets are referenced, not committed. Create them outside the repository before
starting pods:

```bash
kubectl -n velox create secret generic velox-postgres-secret --from-literal=password=...
kubectl -n velox create secret generic velox-auth-secret --from-literal=issuer=... --from-literal=audience=...
kubectl -n velox create secret generic velox-kafka-signing-secret --from-literal=key=...
```

## Commands

```bash
make proto-check
make db-check
make k8s-check
make deploy-dry-run
make deploy
```

The deploy script uses `rtk kubectl` by default and accepts `--dry-run`. It
applies manifests to an existing Kubernetes context; full `kind` cluster
creation, local image loading, and frontend port-forward orchestration are still
follow-up runtime automation work.
