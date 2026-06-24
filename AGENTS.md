# Repository Guidelines

## Project Structure & Module Organization

Project Velox is specified as a high-scale event ticket marketplace using Svelte 5, Go, Rust, Kafka, and isolated persistence per service. Keep product and architecture documents in `docs/`. Expected implementation layout:

- `apps/web/`: Svelte 5 frontend, Runes state, WebSocket/SSE clients, seat-map rendering.
- `services/order-go/`: Go HTTP/gRPC ingress, JWT validation, PostgreSQL outbox, payment orchestration.
- `services/inventory-rust/`: Rust Tokio inventory validator, event store integration, Kafka consumers/producers.
- `services/projection-go/`: Kafka projection workers and read APIs backed by Elasticsearch or MongoDB.
- `infra/`: Kafka, PostgreSQL, Redis, Elasticsearch/MongoDB, observability, and local compose manifests.

## Build, Test, and Development Commands

No executable project scaffolding exists yet. Add commands here when services are created. Use the local `rtk` wrapper for shell commands:

```bash
rtk npm run dev        # Svelte app
rtk go test ./...      # Go services
rtk cargo test         # Rust services
rtk docker compose up  # Local infrastructure, once added
```

## Coding Style & Naming Conventions

Use stack-native formatting: `prettier`/`eslint` for Svelte and TypeScript, `gofmt`/`go vet` for Go, and `rustfmt`/`clippy` for Rust. Use `kebab-case` for docs and frontend assets, idiomatic Go package names, and `snake_case` Rust modules. Kafka topic and event names should be explicit, for example `order.events.v1` and `SeatReservationHeld`.

## Architecture & Service Boundaries

Keep services independently deployable and stateless. State shared across requests or replicas belongs in PostgreSQL, Redis, Kafka, the Inventory event store, or another durable shared system. The Go Order Service owns public command ingress, authentication boundaries, request limits, idempotency checks, and transactional outbox writes. The Rust Inventory Service owns seat availability, reservation expiry, event-sourced stream versions, and double-allocation prevention. Projection services own read-model materialization and must not become write-path authorities.

Treat API, gRPC, and Kafka messages as transport DTOs. Map them deliberately at service boundaries instead of leaking generated or wire-format types into domain code. Change event schemas compatibly: preserve field meaning, add rather than repurpose fields, version breaking changes, and keep consumers tolerant of duplicates. Every network call needs an explicit timeout and safe error mapping. Retries require bounded policy and an operation known to be idempotent.

## Engineering Standards

Follow SOLID, KISS, DRY, YAGNI, and the Pareto principle. Keep changes focused and do not build for hypothetical requirements. Search for an existing helper, abstraction, or platform primitive before adding one. Add abstractions only when they remove concrete complexity or duplication.

Match surrounding structure, naming, and idioms so the codebase reads as one system. Use precise names and standard initialisms such as `ID`, `URL`, `HTTP`, `DB`, and `JWT`. Prefer clarity over compressed code and named constants over repeated policy-significant literals. Keep related fixes together, but do not expand a task into unrelated cleanup. Comments should explain constraints, invariants, security decisions, or non-obvious intent; do not narrate straightforward code or preserve implementation history.

## Secure Engineering

Security controls are design constraints, not review-time additions. Validate untrusted data where it enters the system. Bound request bodies, WebSocket messages, Kafka payloads, pagination, collection sizes, and stream reads before parsing or allocation.

Authentication and authorization default to deny. Never trust client-supplied user IDs, prices, seat states, reservation expiry, or fee totals. Derive identity from validated tokens and keep ownership checks next to protected operations. Use parameterized SQL exclusively. Make check-then-act operations atomic with transactions, constraints, event-store expected versions, Redis `SETNX`, or another durable coordination primitive.

Keep secrets out of code, committed configuration, URLs, browser storage, generated artifacts, and logs. Use established cryptographic primitives and constant-time comparisons for credentials, MACs, signed Kafka events, and tokens. Never render user-controlled HTML directly. Log structured operational metadata without credentials, session values, payment data, request bodies, unnecessary personal data, or raw user-controlled text. Justify new dependencies by maintenance, security, image-size, and runtime cost.

## Go Conventions

Keep handwritten Go `gofmt`-clean. Use symbolic `http.Status*` constants and JSON error responses for public APIs unless an endpoint intentionally streams bytes. Propagate `context.Context` through request, Kafka, Redis, and database boundaries. Do not retain request contexts in work intended to outlive the request. Use typed fakes and `httptest` for transport behavior. Keep database behavior behind narrow interfaces where practical.

## Rust Conventions

Keep handwritten Rust `rustfmt`-clean and address Clippy findings in changed code unless a narrowly scoped suppression documents a generated-code or API constraint. Map internal, storage, Kafka, and validation failures to deliberate public statuses; never expose raw SQL, filesystem, broker, or internal error details to clients. Background tasks must have intentional lifetimes, recover or surface failures at their boundary, and shut down cleanly where required.

## Testing Guidelines

Unit-test deterministic domain rules, especially seat reservation version checks, idempotency handling, outbox publishing, signed-event verification, and projection idempotency. Add integration tests for Kafka saga flows, compensating paths, DLQ handling, and projection lag. Name tests after observable behavior, such as `TestRejectsDuplicateIdempotencyKey` or `rejects_version_mismatch_for_held_seat`. Write behavior-oriented tests for critical paths and risky failure modes; do not chase coverage percentages.

## Commit & Pull Request Guidelines

There is no committed history yet. Use Conventional Commits in imperative present tense: `type(scope): description`. Include a scope when it adds useful context, for example `feat(inventory): add seat stream version check`. Commit messages should be one line, at most 72 characters, with no body, trailers, or issue references. Keep one logical change per commit; tests required by a behavior change belong in the same commit. Review the staged diff before committing, and create commits only when explicitly requested.

PRs should include a summary, verification steps, linked issue, migration notes, rollout or compatibility concerns, and UI screenshots or recordings when frontend behavior changes.

## Definition of Done

Before reporting a change complete:

1. Identify touched untrusted inputs, validation, and resource bounds.
2. Confirm authentication, authorization, and ownership checks.
3. Confirm concurrent or cross-replica operations are atomic and retried work is idempotent.
4. Confirm network calls have timeouts, bounded reads, and deliberate retries.
5. Add or update behavior-oriented tests for critical success and failure paths.
6. Update the relevant spec in `docs/` for new endpoints, schemas, rules, security controls, or infrastructure behavior.
7. Review the complete diff for correctness, security, unnecessary complexity, duplication, stale comments, and unrelated changes.
8. Run relevant formatters, linters, tests, and builds in proportion to risk.
9. Report checks that could not run and remaining risk explicitly.

## Agent-Specific Instructions

Do not overwrite user work. Check file existence before creating artifacts, keep changes scoped, and update `README.md` plus `docs/` when architecture decisions change. Before making code changes, read the relevant spec in `docs/`. Treat flash-sale correctness, idempotency, and Kafka event compatibility as first-class concerns.
