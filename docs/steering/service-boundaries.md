# Service Boundary Rules

Monorepo does not mean monolith. Source-code proximity is never permission to
cross a service boundary through implementation details.

## Source Dependencies

Never import or copy implementation code from another service.

Allowed dependencies:

- A service to generated modules in `contracts/java/` or `contracts/go/`.
- A service to a genuinely cross-cutting infrastructure module.
- BFF or another service to an approved protobuf contract.

Forbidden dependencies:

- One service to another service's controller, handler, domain, repository,
  entity, configuration, or internal package.
- A shared module to service-owned domain code.
- The frontend to backend implementation modules.

If a consumer needs behavior owned elsewhere, expose that behavior through an
approved contract instead of moving the implementation into a shared location.

`make check-boundaries` enforces the import, build-dependency, foreign-database,
internal-Kong-call (`:8000`), and contract/frontend import rules and runs in CI.

## Communication Paths

Choose communication by consistency and response requirements:

- Use direct private gRPC when the caller requires an immediate result.
- Use RabbitMQ events when immediate consistency is unnecessary and the
  publisher should not depend on consumer availability.
- Use scheduled integration jobs only for intentionally delayed reconciliation
  or batch exchange.

Internal services call each other directly on the private network. They do not
call through Kong, BFF, or another service's HTTP/`grpc-gateway` route. Kong
remains the public ingress, and BFF remains an edge composition surface rather
than an internal message bus.

## Cross-service Workflows

Keep an invariant and its state transition inside the owning service. For a
workflow spanning services:

1. Persist the local state change atomically.
2. Publish or call through an approved contract.
3. Make handlers safe for retry and duplicate delivery.
4. Record enough correlation and outcome state to diagnose partial progress.
5. Use compensating actions or an explicit saga/workflow when a later step can
   fail after an earlier service commits.

Prefer asynchronous workflows when the initiating request does not need the
downstream result immediately. Use synchronous calls deliberately: define
timeouts, retry eligibility, failure mapping, and what the caller does when the
dependency is unavailable.

## Data Across Boundaries

Small read-model duplication is acceptable when a service needs local facts for
availability or query performance. Duplicated data must have:

- A clear source-of-truth owner.
- A contract for updates.
- Defined freshness expectations.
- Reconciliation or recovery behavior when delivery fails.

Do not perform runtime joins across service databases. Do not expose persistence
entities as integration contracts. Contract payloads express facts needed by
consumers without granting ownership of the source data.

## Transactions and Consistency

Database transactions end at one service boundary. Multi-service workflows use
local transactions plus one or more of:

- Transactional outbox publication.
- Idempotency keys and request fingerprints.
- Duplicate-safe event consumers.
- Bounded retries with backoff for transient failures.
- Compensating actions.
- Saga or workflow coordination when state spans several services.

Do not hold a database transaction open across a network call. Do not assume
exactly-once delivery. Make repeated commands/events either return the original
outcome or produce no additional side effect.

Clocks are not shared: each service host, each database, and the Docker Desktop
VM that runs local and Testcontainers databases keep their own time, which can
differ by seconds. Write and compare a timestamp with the same clock:

- When a query selects rows by `<column> <= now()`, set that column with the
  database clock too (the column default, or `now() + interval` in SQL), not an
  application `Instant.now()` or `time.Now()`. An outbox row stamped
  `next_attempt_at` by an application clock running ahead of the database stays
  invisible to the relay until the database clock catches up; in tests this
  shows up as an intermittent failure.
- When one service sets a deadline that another service acts on, the two
  services compare it against different clocks. Let one owner decide the
  expiry, or give the other side a grace margin larger than the expected skew.

## Boundary Review Checklist

Before approving a cross-service change, confirm:

- The owning service still decides every affected business invariant.
- No implementation import or database access crosses the boundary.
- Synchronous versus asynchronous communication is justified.
- Contract compatibility and consumer impact are analyzed.
- Retry, idempotency, timeout, and compensation behavior are explicit.
- Due times and deadlines are written and compared with the same clock.
- Logs and traces carry correlation identifiers across the workflow.
