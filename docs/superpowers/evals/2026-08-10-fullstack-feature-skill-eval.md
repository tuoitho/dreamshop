# Fullstack Feature Skill Evaluation

## Baseline

### Scenario 1
- Raw decision:

````text
Affected surfaces

- `contracts/`: stock-reservation protobuf request/response, status/error enum, regenerated Java/Go clients.
- `services/inventory-service/` (Go): atomic reserve/release/confirm stock operations, persistence and idempotency.
- `services/order-service/` (Maven/Spring): order lifecycle orchestration, inventory client, compensation on failure/cancellation.
- REST boundary: order creation/status endpoints expose reservation outcome without leaking inventory internals.
- `ecommerce-fe/` (React/Vite): checkout availability/error state and order-status presentation.
- Operational configuration: inventory endpoint/routing, timeouts, retry policy, observability fields.

Interfaces

- Protobuf boundary: `ReserveStock(order_id, idempotency_key, items[]) → reservation_id, status, expires_at, unavailable_items[]`; plus `ConfirmReservation` and `ReleaseReservation`.
- Inventory behavior: reserve must be atomic per SKU, idempotent by order/key, reject insufficient quantity deterministically, and prevent negative available stock.
- Order REST behavior: create order returns a stable order ID and a distinct stock-unavailable response; confirmation triggers reservation confirmation; cancellation/payment failure triggers release.
- Frontend contract: checkout submits cart lines, handles unavailable-item detail, and never treats an unconfirmed reservation as a completed order.

Execution mode

- Contract-first, TDD-gated, backend-first: define protobuf and generated clients; implement inventory tests and service; implement order orchestration and API tests; then React behavior.
- Use a reservation state machine: `PENDING → RESERVED → CONFIRMED`, with `RELEASED` and `EXPIRED` terminal paths.
- Keep cross-service calls idempotent; use bounded retries only for transport failures, not business rejections.
- Add an expiry/reconciliation path so abandoned reservations return stock.

Role assignments

- Contract owner: specify proto messages, validation, compatibility rules, regenerate clients, run `buf lint`.
- Inventory owner: implement atomic stock mutation, reservation storage, expiration job, Go unit/integration tests.
- Order owner: implement Spring orchestration, transactional order-state changes, compensation, REST tests.
- Frontend owner: add checkout/status UX and React tests using mocked BFF responses.
- QA/integration owner: run end-to-end success, insufficient-stock, duplicate-request, cancellation, expiration, and concurrent-reservation scenarios.
- Reviewer: verify state transitions, idempotency keys, timeout/compensation behavior, and protobuf backward compatibility.

Verification commands

```powershell
buf lint
buf generate
go test ./services/inventory-service/...
mvn -pl services/order-service test
npm --prefix ecommerce-fe run test
npm --prefix ecommerce-fe run lint
npm --prefix ecommerce-fe run build
```

```powershell
docker compose -f deployments/docker/compose.foundation.yml up -d
```

- Execute integration tests against running foundation services, including concurrent reservations for the same SKU and a forced timeout/cancellation compensation case.
````

- Failure pattern: Prescriptive multi-role decomposition and execution mode: the response assigns six discrete owners and requires a contract-first, TDD-gated, backend-first flow. It does not explicitly require one backend agent or an OpenAPI file.

### Scenario 2
- Raw decision:

````text
Affected surfaces

- Web application: account-event producer, validation, UI/error states, event publishing.
- Shared schema: versioned account-event contract and compatibility rules.
- Python worker: consumer, idempotent processing, retries/dead-letter handling, observability.
- Event infrastructure: topic/queue configuration, permissions, retention.

Interfaces

- `AccountEvent` envelope: `eventId`, `eventType`, `schemaVersion`, `occurredAt`, `accountId`, `payload`, `correlationId`.
- Versioned schema published as the source of truth; web producer and Python consumer generated/validated from it.
- Delivery semantics: at-least-once; worker deduplicates by `eventId`.
- Failure contract: retryable vs. terminal errors, with terminal messages sent to a dead-letter queue.

Execution mode

- Contract-first and TDD-gated: define schema and compatibility tests first, then producer and worker tests, then implementation.
- Use an outbox/publisher boundary in the web application so account writes and event creation remain atomic.
- Roll out consumers before enabling production publishing; monitor lag, duplicate handling, failures, and DLQ volume.

Role assignments

- Product/architect: event types, payload meaning, acceptance criteria, retention/privacy rules.
- Web engineer: account mutation flow, outbox, schema validation, producer tests.
- Python engineer: consumer, idempotency store, retry/DLQ behavior, worker tests.
- Platform engineer: broker topics, DLQ, credentials, dashboards, alerts.
- QA: schema compatibility, end-to-end flow, duplicate/retry/failure scenarios.

Verification commands

```powershell
npm --prefix ecommerce-fe run test
npm --prefix ecommerce-fe run lint
pytest
buf lint
buf generate
docker compose -f deployments/docker/compose.foundation.yml up -d
```
````

- Failure pattern: Unsupported tooling and project-path assumptions: the response selects `buf`/protobuf generation and the `ecommerce-fe` test commands although the raw scenario names only a web application, Python worker, and event schema. It also prescribes five specialized roles and a contract-first, TDD-gated flow.

### Scenario 3
- Raw decision:

````text
Affected surfaces

- Backend calculation/domain service and its focused unit tests.
- No frontend, API contracts, protobuf, database schema, gateway, or deployment changes.

Interfaces

- Preserve all public REST/gRPC/message interfaces, request/response models, and error contracts.
- Restrict changes to internal calculation logic and private helper boundaries.

Execution mode

- Backend-only, contract-preserving change.
- Test-first: add regression cases for the revised calculation, implement the smallest internal change, then run focused and service-level tests.

Role assignments

- Architect: define calculation rule, invariants, and edge cases.
- QA: specify expected-result test matrix, including boundaries and regressions.
- Backend engineer: implement internal logic and tests.
- Reviewer: confirm public-interface compatibility and test evidence.

Verification commands

```powershell
mvn test
go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
```
````

- Failure pattern: No baseline failure demonstrated. The response keeps scope backend-only and lists four role assignments, as requested, but does not demonstrate that delegation is mandatory.

## Forward Evaluation

Pending until the revised skill exists.
