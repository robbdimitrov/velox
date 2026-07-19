# Infrastructure

## Cache Stampede on Hot Stadium Layouts

Scenario: 100,000 users request the same seat layout at sale open.

Failure mode:

- Read API saturates projection store.
- CDN misses align at the same second.
- Live fanout repeatedly serializes identical snapshots.

Required controls:

1. Pre-warm event and venue layout documents before sale open.
2. Split static geometry from dynamic seat state.
3. Cache static geometry with long TTL and immutable content hashes.
4. Cache dynamic section snapshots with 250 ms to 1 second TTL plus
   stale-while-revalidate.
5. Use Redis single-flight locks:

```text
SET hotload:{event_id}:{section_id} worker_id NX PX 1000
```

Only the lock holder refreshes the projection snapshot. Other requests receive
stale data that is marked with `snapshot_age_ms`.

Live-update rule: broadcast deltas, not full maps. A delta must include only
changed seats:

```json
{
  "event_id": "evt_123",
  "section_id": "A",
  "changes": [{ "seat_id": "A-12", "status": "HELD", "version": 3 }]
}
```

## Poison Pills and DLQ Management

Scenario: a corrupt or incompatible Kafka payload blocks a consumer partition.

Required controls:

- Validate schema at the consumer boundary.
- Use a schema registry and reject unknown required fields for the active major
  version.
- Limit retries for deterministic failures.
- Send unrecoverable records to a DLQ topic without committing false business
  success.

Topic pattern:

```text
order.events.v1
inventory.events.v1
dlq.order.events.v1
dlq.inventory.events.v1
```

DLQ payload must include:

```text
source_topic
source_partition
source_offset
consumer_group
error_class
error_message
payload_hash
first_seen_at
last_seen_at
correlation_id
```

Rules:

- Do not let one corrupt message stall unrelated partitions.
- For invalid schemas, commit the source offset only after the DLQ write is
  acknowledged.
- For transient infrastructure failures, retry with backoff and do not DLQ
  immediately.
- Alert on DLQ rate, repeated payload hashes, and schema-version mismatches.
- Provide replay tooling that can reprocess fixed DLQ records into a quarantine
  topic before production topics.

## Distributed Clock Skew and Reservation Expiry

Scenario: reservations expire after 10 minutes, but service clocks differ by
milliseconds.

Failure mode:

- Client shows time remaining after backend considers hold expired.
- Two services disagree on timeout ordering.
- Expiry workers release seats too early or too late.

Required controls:

1. `seatservice` owns reservation deadlines.
2. `SeatReservationHeld` stores `expires_at_server_ms` computed by `seatservice`
   using a monotonic clock plus persisted wall timestamp.
3. Clients display countdown from server time offset, but cannot extend or
   validate holds.
4. Expiry append uses compare-and-append:

```text
append SeatReservationExpired only if latest event is SeatReservationHeld and now >= expires_at
```

5. Reservation confirmation after expiry must be rejected unless a later valid
   hold exists.

Operational rules:

- Run NTP or chrony on all nodes and alert on skew above 50 ms.
- Never rely on Kafka event arrival time for expiry.
- Persist deadlines in the event payload and projection document.

## Partition Ordering and Hot Seats

Kafka partition keys must preserve ordering for a seat aggregate:

```text
key = event_id + ":" + section_id + ":" + seat_id
```

This ensures `SeatReservationHeld`, `SeatReservationExpired`, and
`SeatReservationConfirmed` are consumed in sequence for the same seat. For
event-wide projections, consumers must handle cross-seat ordering as eventually
consistent.

## Projection Lag and User Experience

Projection lag is expected under flash-sale load. The command path must return
authoritative reservation results from write services, while the UI read model
may lag.

Rules:

- Include `projection_lag_ms` in read API metadata.
- Seat selector must favor live deltas over stale snapshot reads.
- Checkout must validate reservation tokens against command services, not
  projections.
- If lag exceeds the configured threshold, freeze risky actions and show stale
  status indicators.

## Duplicate and Out-of-Order Events

Every consumer must be idempotent.

Required consumer table or document fields:

```text
event_id
aggregate_id
aggregate_version
processed_at
```

Rules:

- Drop duplicate `event_id`.
- Reject lower aggregate versions.
- Buffer or retry missing intermediate versions when strict sequence is
  required.
- Wallet issuance must not depend on order-summary projection winning a race
  against inventory confirmation. Add durable pending-ticket buffering,
  command-side ticket identity, or bounded reconciliation before treating
  wallet tickets as complete.
- Never apply reservation confirmation, ticket issuance, or transfer effects
  from an unsigned event.

## Backpressure and Load Shedding

Ingress must preserve correctness over availability for mutation endpoints.

Rules:

- Discovery endpoints can serve stale cached data.
- Reservation endpoints return `429` or `503` when Redis, Kafka, or
  `seatservice` health is degraded.
- Checkout must fail closed if idempotency storage is unavailable.
- Live gateways should drop nonessential ticker messages before seat-state
  messages.

## Dependency Connection Lifetimes and Probes

Services that pool connections to restartable dependencies must bound pooled
connection reuse. PostgreSQL, Redis, and similar clients should configure max
connection lifetime and idle lifetime instead of relying on library defaults.

Application probes are split by purpose:

- `/healthz` is process liveness and must stay lightweight.
- `/readyz` checks dependencies required to serve traffic plus local background
  pipeline health where the service owns a consumer or relay.
- Kubernetes readiness probes use `/readyz`; liveness and startup probes use
  `/healthz`.

Background relays and consumers must expose enough process-local state for
readiness to detect repeated infrastructure failures without treating an empty
topic or outbox as unhealthy.

Consumer and relay readiness uses a sustained-failure policy. A transient poll
or publish error is recorded, but `/readyz` fails only after five consecutive
errors or after the first unrecovered error remains present for 30 seconds.
Successful progress clears the error streak and the failure window.
`orderservice` and `viewservice` expose the same pipeline and consumer health as
Prometheus text metrics on `/metrics`, including consecutive errors, unhealthy
state, and age since last success or first unrecovered error. Metrics include
the existing `velox_*` names plus canonical `app_pipeline_*` aliases with labels
`app="velox"`, `service`, and `pipeline`.

Run `scripts/failure-drill.sh` after changing dependency clients, probes, or
consumer loops. The script restarts PostgreSQL, cache, and broker workloads,
then waits for the application deployments to remain rolled out. Override the
namespace and wait budget with `NAMESPACE=...` and `TIMEOUT=...`.

## Pod Identity and Hardening

Every workload — application services, `frontend`, `database`, `cache`,
`broker`, and `broker-topics-init` — runs under its own `ServiceAccount` with
`automountServiceAccountToken: false`; none of them call the Kubernetes API.
Pods run as a non-root UID matching the container image's native user
(`65532` for the Go/Rust services, `1000` for `frontend`, `70` for
`database`, `999` for `cache`, `101` for `broker` and
`broker-topics-init`), drop all Linux capabilities, and deny privilege
escalation.

`readOnlyRootFilesystem` is enabled everywhere except:

- `database`: PostgreSQL writes PGDATA and socket/PID files.
- `broker` and `broker-topics-init`: `rpk` regenerates
  `/etc/redpanda/redpanda.yaml` on start; mounting that path as a separate
  volume breaks its advertised-listener validation for StatefulSet pod
  hostnames.

`frontend` and `cache` mount a writable `emptyDir` for `/tmp` and `/data`
respectively to keep `readOnlyRootFilesystem` enabled.

## Network Policy

The namespace uses default-deny ingress and egress policies. Every pod may
resolve DNS through `kube-system` CoreDNS, and explicit egress rules mirror the
current service graph:

- `frontend` may call `apigateway`.
- `apigateway` may call `orderservice`, PostgreSQL, and Redis.
- `orderservice`, `seatservice`, and `viewservice` may call PostgreSQL and
  Kafka.
- `broker-topics-init` may call Kafka during topic provisioning.
- `broker` may call its own broker/admin ports for single-node Redpanda
  operation.

When adding a new synchronous dependency or background client, update
`deploy/networkpolicy.yaml` in the same change as the workload manifest.
