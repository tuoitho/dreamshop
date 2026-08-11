# Change Impact Analysis

Classify the affected scope before implementation. The classification is an
inventory for reasoning and review; it is not permission to modify every file or
consumer discovered.

## Impact Classes

### DIRECT

Files required to implement the requested behavior inside the owning surface.

Examples:

- An Identity application service and its controller.
- An Inventory reservation rule.
- A frontend page and its API adapter.

### CONTRACT

Compatibility surfaces shared with callers or consumers.

Examples:

- Protobuf messages, RPCs, generated modules, or HTTP annotations.
- REST paths, request/response fields, envelopes, status codes, and OpenAPI.
- Event type, version, metadata, payload, routing key, or error reason.
- Public DTOs or configuration interfaces consumed outside their owner.

### DEPENDENT

Applications that call or consume a changed contract.

Examples:

- BFF consuming a Catalog RPC.
- Inventory consuming an Order event.
- Frontend consuming an Identity REST response.

Inspect dependent code and tests for compatibility. Do not modify a dependent
surface when the change is additive and its existing behavior remains valid.

### INFRA

Operational resources needed by the behavior.

Examples:

- Flyway migrations, database initialization, or retained data.
- Environment variables, secret references, ports, and service discovery.
- Kong routes/plugins, Docker Compose, RabbitMQ topology, or Redis usage.
- Health checks, dashboards, alerts, deployment order, or rollback resources.

### TEST

Evidence required to validate the behavior and its boundaries.

Examples:

- Unit tests for an invariant or state transition.
- Repository or adapter integration tests.
- Producer/consumer contract tests.
- Focused frontend behavior tests.
- A critical cross-service end-to-end flow.

## Analysis Procedure

Before changing files:

1. Name the owning service or application surface.
2. List `DIRECT` files needed for the smallest implementation.
3. List every `CONTRACT` touched, including error and compatibility semantics.
4. Identify `DEPENDENT` consumers and whether inspection or modification is
   required.
5. Identify `INFRA` changes, rollout order, and recovery concerns.
6. Select `TEST` evidence from the narrowest scope outward.
7. Remove unrelated files and speculative refactors from the proposed scope.

If the work is completely owned by one service and no contract changes, keep the
change in that service. If a contract changes, inspect all known consumers before
editing them. If compatibility can be preserved additively, prefer that over a
coordinated breaking rollout.

## Expected Handoff

A change summary should distinguish:

- Files actually changed in each class.
- Dependents inspected but intentionally unchanged.
- Compatibility or rollout assumptions.
- Verification commands run and their outcomes.
- Environmental blockers that prevented a broader check.

