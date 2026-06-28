# Velox

## Architecture

Velox is a high-scale event ticket marketplace using SvelteKit SSR with Svelte
5, Tailwind, DaisyUI, Lucide icons, Go, Rust, Kafka, and isolated persistence
per service. Keep product and architecture documents in `docs/`.

- `apps/frontend/` — SvelteKit SSR frontend with Svelte 5, Tailwind, DaisyUI,
  Lucide, Runes state, WebSocket/SSE clients, seat-map rendering.
- `apps/apigateway/` — Go public HTTP API, auth boundary, rate limiting,
  validation, and gRPC orchestration.
- `apps/orderservice/` — Go order state, idempotency, reservation tokens,
  payments, and PostgreSQL outbox.
- `apps/seatservice/` — Rust Tokio inventory validator, event store integration,
  Kafka consumers/producers.
- `apps/viewservice/` — Go Kafka projection workers, read APIs, and
  WebSocket/SSE fanout.
- `apps/database/` — PostgreSQL migrations and database bootstrap assets.
- `pkg/` — shared generated transport contracts such as `pkg/pb`.
- `deploy/` — Kafka, PostgreSQL, Redis, Elasticsearch/MongoDB, observability,
  and application manifests.
- `scripts/` — local development, deployment, and maintenance automation.

## Commands

```sh
make test          # Go and Rust service tests
make lint          # backend, frontend, script, and schema checks
make build         # backend and frontend builds
scripts/deploy.sh  # build images, apply manifests, and port-forward
```

## Contracts and Service Boundaries

- Keep services independently deployable and stateless. State shared across
  requests or replicas belongs in PostgreSQL, Redis, Kafka, the Inventory event
  store, or another durable shared system.
- `apigateway` owns public command ingress, authentication boundaries, request
  limits, and safe error mapping. `orderservice` owns order state, idempotency
  checks, payment orchestration, and transactional outbox writes. `seatservice`
  owns seat availability, reservation expiry, event-sourced stream versions, and
  double-allocation prevention. `viewservice` owns read-model materialization
  and must not become a write-path authority.
- Treat API, gRPC, and Kafka messages as transport DTOs. Map them deliberately
  at service boundaries; do not leak generated or wire-format types into domain
  code.
- Change event schemas compatibly: preserve field meaning, add rather than
  repurpose fields, version breaking changes, and keep consumers tolerant of
  duplicates.
- Every network call needs an explicit timeout and safe error mapping. Retries
  require a bounded policy and an idempotent operation.
- Treat flash-sale correctness, idempotency, and Kafka event compatibility as
  first-class concerns.

## Engineering Standards

- Follow SOLID, KISS, DRY, YAGNI, and the Pareto principle. Keep changes
  focused; do not build for hypothetical requirements.
- Search for an existing helper, abstraction, or platform primitive before
  adding one. Add abstractions only when they remove concrete complexity or
  duplication.
- Match surrounding structure, naming, and idioms so the codebase reads as one
  system.
- Use precise names and standard initialisms. Prefer clarity over compressed
  code and named constants over repeated policy-significant literals.
- Keep related fixes together; do not expand a task into unrelated cleanup.
- Comments explain constraints, invariants, security decisions, or non-obvious
  intent. Do not narrate straightforward code or preserve implementation
  history.
- Do not suppress compiler, linter, type-checker, or test warnings to make
  checks pass. Fix the underlying issue. Use a narrowly scoped suppression only
  when required by an external API, generated code, or a documented false
  positive, and explain why it is safe.
- Write behavior-oriented tests for critical paths, complex logic, and risky
  failure modes. Do not chase coverage percentages.

## Secure Engineering

Security controls are design constraints, not review-time additions.

- Validate untrusted data where it enters the system. Bound request bodies,
  WebSocket messages, Kafka payloads, pagination, collection sizes, and stream
  reads before parsing or allocation.
- Authentication and authorization default to deny. Never trust client-supplied
  user IDs, prices, seat states, reservation expiry, or fee totals. Derive
  identity from validated tokens and keep ownership checks next to protected
  operations.
- Use parameterized SQL exclusively. Make check-then-act operations atomic with
  transactions, constraints, event-store expected versions, Redis `SETNX`, or
  another durable coordination primitive.
- Keep secrets out of code, committed configuration, URLs, browser storage,
  generated artifacts, and logs.
- Use established cryptographic primitives and constant-time comparisons for
  credentials, MACs, signed Kafka events, and tokens. Do not invent
  cryptographic protocols.
- Never render user-controlled HTML directly. Validate user-controlled URLs
  against an explicit scheme and origin policy.
- Log structured operational metadata without credentials, session values,
  payment data, request bodies, unnecessary personal data, or raw
  user-controlled text.
- Justify new dependencies by their maintenance, security, image-size, and
  runtime cost.

## Go Conventions

- Keep handwritten code `gofmt`-clean and use standard initialisms such as `ID`,
  `URL`, `HTTP`, `DB`, and `JWT`.
- Use symbolic `http.Status*` constants. The public API contract is JSON,
  including errors, unless an endpoint intentionally streams bytes.
- Propagate `context.Context` through request, Kafka, Redis, and database
  boundaries. Do not retain request contexts in work intended to outlive the
  request.
- Use typed fakes and `httptest` for transport behavior. Keep database behavior
  behind narrow interfaces where practical.

## Rust Conventions

- Keep handwritten code `rustfmt`-clean and address Clippy findings in changed
  code unless a narrowly scoped suppression documents a generated-code or API
  constraint.
- Map internal, storage, Kafka, and validation failures to deliberate public
  statuses. Do not expose raw SQL, filesystem, broker, or internal error details
  to clients.
- Background tasks must have intentional context lifetimes, recover or surface
  failures at their boundary, and shut down cleanly where required.

## Testing

- Unit-test deterministic domain rules, especially seat reservation version
  checks, idempotency handling, outbox publishing, signed-event verification,
  and projection idempotency.
- Add integration tests for Kafka saga flows, compensating paths, DLQ handling,
  and projection lag.
- Name tests after observable behavior, such as
  `TestRejectsDuplicateIdempotencyKey` or
  `rejects_version_mismatch_for_held_seat`.
- Aim for Pareto 80/20 test coverage focusing on critical control surfaces (e.g., Auth controllers, vendor actions, and core read-paths) rather than exhaustive line coverage.
- Keep Go API unit tests fast and hermetic by passing a `nil` store or using lightweight mocks instead of requiring live PostgreSQL instances.

## Resilience

- Hard dependencies (database, required backends) must be available at startup —
  fail fast if they are not. Soft dependencies (cache, rate limiter, search)
  must not block startup; start degraded and self-heal.
- Self-heal soft dependencies: attempt a synchronous connection first, then on
  failure retry in the background with exponential backoff and log on entering
  degraded mode and on recovery. The client library owns reconnection once the
  initial connection succeeds.
- Prefer client primitives that survive reconnection over state computed at
  startup. Do not cache server-side state that is lost when a connection resets.

## Kubernetes Resources

- Set CPU requests on every container; omit CPU limits. CFS quota throttles at
  the limit even when the node has spare cycles, causing latency spikes.
- Always set memory requests and limits. Memory is not compressible — OOM kill
  is preferable to silently exhausting node memory.
- Pin third-party images to specific versions. Never use `:latest`.

## Git and Commits

- Keep one logical change per commit. Tests required by a behavior change belong
  in the same commit.
- Use Conventional Commits (`type(scope): description`) in imperative present
  tense. Include a scope when it adds useful context.
- Commit messages are one line, at most 72 characters, with no body, trailers,
  or issue references.
- Review the staged diff before committing. Create commits only when the user
  explicitly requests them.
- PRs should include a summary, verification steps, migration notes, rollout or
  compatibility concerns, and UI screenshots or recordings when frontend
  behavior changes.

## Definition of Done

Before reporting a change complete:

1. Identify touched untrusted inputs, validation, and resource bounds.
2. Confirm authentication, authorization, and ownership checks.
3. Confirm concurrent or cross-replica operations are atomic and retried work is
   idempotent.
4. Confirm network calls have timeouts, bounded reads, and deliberate retries.
5. Add or update behavior-oriented tests for critical success and failure paths.
6. Review the complete diff for correctness, security, unnecessary complexity,
   duplication, stale comments, and unrelated changes.
7. Run relevant formatters, linters, tests, and builds in proportion to risk.
8. Report checks that could not run and remaining risk explicitly.

## Specs

- Before making any code changes, read the relevant spec in `docs/`.
- Update the relevant spec to reflect the change — new endpoints, schema
  changes, rule additions, security controls, or infrastructure modifications —
  before marking work complete.
