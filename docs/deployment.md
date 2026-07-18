# Deployment Support

Velox deployment artifacts include runtime images, database initialization,
Kubernetes manifests, and a local deploy script.

## Protobuf

The reservation MVP transport contract lives in `pkg/pb/velox.proto`. It covers:

- reservation creation, reservation confirmation, and order lookup RPCs;
- signed Kafka event envelopes with correlation, causation, aggregate version,
  schema version, and occurrence time metadata;
- order, reservation, ticket, and seat-delta messages used by the order, seat,
  and view services.

Generated language bindings are not committed.

## Database

`apps/database/migrations/001_init_logical_schemas.sql` creates three logical
PostgreSQL schemas:

- `orders`: order state, idempotency keys, and transactional outbox rows;
- `inventory`: seat event streams, immutable inventory events, and reservation
  expiry state;
- `projection`: processed-event dedupe, seat snapshots, order summaries, and
  wallet tickets.

`apps/database/seeds/999_demo_reservation_mvp.sql` adds demo reservation data
for local smoke testing. The database image copies migrations and seeds into
Postgres' `docker-entrypoint-initdb.d`, so a fresh volume initializes on first
startup.

## Kubernetes

Manifests live in `deploy/` and create:

- per-workload ServiceAccounts;
- Database StatefulSet and service;
- Broker StatefulSet and service;
- Cache Deployment and service;
- service Deployments for `frontend`, `apigateway`, `orderservice`,
  `seatservice`, and `viewservice`.

No manifest sets its own namespace. `scripts/deploy.sh` creates the namespace
named by `NS` (default `velox`) and applies every manifest into it with
`kubectl apply -n`. Inter-service addresses use unqualified Service names
(`broker`, `database`, `cache`, `orderservice`), which resolve within whatever
namespace the pods actually run in, so `NS` can be overridden without editing
any manifest.

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
`make`, applies manifests to the current Kubernetes context, waits for staged
rollouts, and starts a frontend port-forward:

- frontend: `http://localhost:8085` by default, configurable with
  `LOCAL_FRONTEND_PORT`

Use `scripts/deploy.sh --dry-run` for client-side manifest validation and
`scripts/deploy.sh --skip-build` when images are already available locally.
Override `KUBECTL` and `IMAGE_PREFIX` to match a non-default cluster or
registry. By default, each image is tagged with a 12-character checksum of its
own build context, so unchanged services keep stable image tags and avoid
unnecessary rollouts. Set `GIT_SHA` to force one shared tag for every image, or
set `APIGATEWAY_IMAGE_TAG`, `ORDERSERVICE_IMAGE_TAG`, `SEATSERVICE_IMAGE_TAG`,
`VIEWSERVICE_IMAGE_TAG`, `FRONTEND_IMAGE_TAG`, or `DATABASE_IMAGE_TAG` for
per-image overrides. Images are pushed to
`IMAGE_PREFIX-<service>:<tag>`; when a `velox-control-plane` kind node is
running, `scripts/deploy.sh` also loads the built images directly into it. Full
`kind` cluster creation is still follow-up automation work.

Deploys apply base manifests first, then infrastructure, provision Kafka topics,
and finally apply app services. Workloads are annotated with checksums for the
Secrets and ConfigMaps they consume, so data-only changes roll the affected pods
without forcing unrelated services to restart.

Frontend container builds use BuildKit's npm cache with `npm ci` against the
committed lockfile in the builder stage and copy only the adapter-node build
output into the Node runner image. Local Rust builds share a repository-level
Cargo `target/` directory so repeated `seatservice` checks reuse artifacts
instead of rebuilding under `apps/seatservice/target`.
