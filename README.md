# Project Velox

Project Velox is a high-scale event ticket marketplace designed for flash-sale traffic spikes where hundreds of thousands of users contend for the same venue inventory at once. The target stack is Svelte 5 on the frontend, Go and Rust backend microservices, Apache Kafka as the event backbone, and isolated databases per service.

## Architecture Summary

Velox uses CQRS and event-driven choreography. Commands enter through the Go Order Service, are persisted with a PostgreSQL transactional outbox, and are streamed to Kafka. The Rust Inventory Service validates seat availability with event-sourced optimistic concurrency and publishes immutable inventory events. Go projection workers flatten Kafka events into Elasticsearch or MongoDB read models consumed by the Svelte UI through fast read APIs, WebSockets, and SSE streams.

```text
Svelte 5 Client
  | commands
  v
Go Order Service -> PostgreSQL Outbox -> CDC -> Kafka
                                             |
                                             v
Rust Inventory Service -> Event Store -> Kafka inventory events
                                             |
                                             v
Go Projection Workers -> Elasticsearch/MongoDB -> Read API/WebSockets
```

## Specification Index

- [Thematic and Functional Blueprint](docs/thematic-functional-blueprint.md)
- [Architecture and Consistency Specification](docs/architecture-consistency-security.md)
- [Operational Edge Cases and Failure Modes](docs/operational-edge-cases.md)

## Initial Service Boundaries

- `apps/web/`: Svelte 5 UI, Runes client state, live event discovery, seat selector, checkout, wallet.
- `services/order-go/`: HTTP/gRPC ingress, JWT validation, rate limiting, idempotency, transactional outbox.
- `services/inventory-rust/`: Tokio Kafka consumers, event store append logic, reservation expiry, seat stream concurrency.
- `services/projection-go/`: Kafka consumers, read-model materializers, WebSocket/SSE fanout.
- `infra/`: Kafka, Redis, PostgreSQL, Elasticsearch or MongoDB, observability, deployment manifests.

## Current Status

This repository currently contains initial documentation only. No runtime service scaffolding, package manifests, CI, or local infrastructure files have been added yet.
