# Velox

**Velox** is a high-scale event ticket marketplace designed for flash-sale traffic spikes where hundreds of thousands of users contend for the same venue inventory at once. The target stack is a Svelte 5 frontend, Go and Rust backend microservices, Apache Kafka as the event backbone, and isolated databases per service.

## Architecture Summary

Velox uses CQRS and event-driven choreography. Commands enter through `apigateway`, order writes are persisted by `orderservice` with a PostgreSQL transactional outbox, and durable events are streamed to Kafka. `inventoryservice` validates seat availability with event-sourced optimistic concurrency and publishes immutable inventory events. `projectionservice` flattens Kafka events into Elasticsearch or MongoDB read models consumed by the Svelte UI through fast read APIs, WebSockets, and SSE streams.

```text
Svelte 5 Client
  | commands
  v
apigateway -> orderservice -> PostgreSQL Outbox -> CDC -> Kafka
                                             |
                                             v
inventoryservice -> Event Store -> Kafka inventory events
                                             |
                                             v
projectionservice -> Elasticsearch/MongoDB -> Read API/WebSockets
```

## Docs

Architectural specs live in [`docs/`](docs/):

| Doc | Contents |
|---|---|
| [architecture.md](docs/architecture.md) | Service topology, consistency model, event choreography, security controls |
| [frontend.md](docs/frontend.md) | UI direction, discovery, seat selection, checkout, wallet flows |
| [infrastructure.md](docs/infrastructure.md) | Operational edge cases, Kafka failure modes, cache behavior, backpressure |

## Initial Service Boundaries

| Component | Language | Description |
|---|---|---|
| `apps/frontend/` | TypeScript | Svelte 5 UI, Runes client state, live event discovery, seat selector, checkout, wallet. |
| `apps/apigateway/` | Go | Public HTTP API, auth boundary, rate limiting, request validation, gRPC orchestration. |
| `apps/orderservice/` | Go | Order state, idempotency, reservation tokens, payment orchestration, transactional outbox. |
| `apps/inventoryservice/` | Rust | Tokio Kafka consumers, event store append logic, reservation expiry, seat stream concurrency. |
| `apps/projectionservice/` | Go | Kafka consumers, read-model materializers, read APIs, WebSocket/SSE fanout. |
| `apps/database/` | PostgreSQL | Versioned schema migrations for service-owned relational stores. |
| `pkg/` | Protobuf | Shared generated transport contracts such as `pkg/pb`. |
| `deploy/` | Kubernetes | Kafka, Redis, PostgreSQL, Elasticsearch or MongoDB, observability, and application manifests. |
| `scripts/` | Shell | Local development, deployment, and maintenance automation. |

## Current Status

This repository currently contains initial documentation only. No runtime service scaffolding, package manifests, CI, or local infrastructure files have been added yet.

## License

Licensed under the [MIT](LICENSE) License.
