# API and Event Contract Rules

Contracts are service boundaries, not shared implementation. Producers and
consumers evolve independently, so compatibility and duplicate delivery must be
designed explicitly.

## Contract Locations

- `contracts/proto/` contains protobuf source definitions.
- `contracts/java/` and `contracts/go/` contain generated language modules.
- Public HTTP contracts are defined by Identity controllers, BFF handlers, and
  `deployments/kong/*.yaml`.

Generated Java and Go contract source is tracked. Do not edit generated files by
hand. Change the protobuf source definition, run `buf format --diff --exit-code`,
`buf lint`, and `buf generate`, then commit the resulting generated output with
the source-contract change. Do not manually normalize generator-produced
whitespace in generated Java source; generation synchronization is authoritative.

## Compatibility

Prefer additive evolution.

Usually compatible:

- Adding an endpoint or RPC without changing existing behavior.
- Adding an event type or a new event version.
- Adding an optional field with a safe absence/default interpretation.
- Adding a new error code without changing existing code meanings.

Potentially breaking:

- Renaming, deleting, or changing the type of a field.
- Changing a field, status, or error code's meaning.
- Reusing a removed protobuf field number or enum value.
- Changing enum semantics or treating an unknown value as impossible.
- Making an optional input required.
- Changing event routing, payload structure, ordering assumptions, or version
  semantics.

Breaking changes require explicit versioning or a staged migration. Document the
producer/consumer rollout order, compatibility window, and removal criteria.
Inspect every known consumer before finalizing the migration.

Consumers must tolerate additional unknown fields. They must handle absent
optional fields and unknown enum values safely. Do not make correctness depend on
all services deploying simultaneously.

## Protobuf and gRPC

- Protobuf fields use `lower_snake_case` and packages remain versioned.
- Never renumber existing fields. Reserve removed field numbers and names.
- Prefer new fields/RPCs over changing established semantics.
- Keep domain RPCs private unless a reviewed `google.api.http` annotation and
  Kong route intentionally expose them.
- Internal callers use direct private gRPC, not Kong, BFF, or a public
  transcoding route.
- Use standard gRPC status codes plus a stable, versioned domain reason, such as
  metadata represented by `google.rpc.ErrorInfo`.

Run:

```powershell
buf format --diff --exit-code
buf lint
buf generate
git diff --exit-code -- contracts/go contracts/java/src/main/generated
```

After generation, compile and test every affected Java and Go producer/consumer.
Generated output must be committed and synchronized with its protobuf source. A
successful generation command does not by itself prove behavioral compatibility.

## REST/JSON

REST/JSON fields use `camelCase`.

Successful response:

```json
{
  "data": {},
  "message": "Operation completed."
}
```

Use `"data": null` when no payload exists. Mutation endpoints return a meaningful
success `message` suitable for client notification.

Paginated response data uses:

```json
{
  "data": {
    "items": [],
    "page": 1,
    "pageSize": 20,
    "totalItems": 0,
    "totalPages": 0
  },
  "message": "Items retrieved."
}
```

Error response:

```json
{
  "code": "STABLE_ERROR_CODE",
  "message": "Human-readable explanation.",
  "errors": {},
  "traceId": "01J..."
}
```

Omit `errors` when no structured details exist. Do not return ad hoc error
shapes, stack traces, database errors, provider details, or internal exception
messages. Business errors use the same envelope and an appropriate non-2xx HTTP
status. Keep `code` stable for client behavior; changing prose in `message` must
not break consumers.

Kong is the public ingress. Identity REST routes may go directly through Kong;
simple approved unary RPCs may use reviewed transcoding; client-specific
composition and custom mapping go through BFF. Public routing must remain aligned
with [ADR-008](../adr/008-hybrid-rest-bff-grpc-transcoding.md).

## Events

Transactional event publication, retry, and consumer deduplication follow
[ADR-002](../adr/002-transactional-outbox.md).

Every event contract defines at least:

- `event_id`: globally unique delivery identity.
- `event_type`: stable semantic name.
- `event_version`: payload/schema version.
- `occurred_at`: producer event time in an unambiguous format.
- `aggregate_id`: stable identity of the domain aggregate/entity that produced
  the fact.
- `correlation_id`: workflow/trace correlation.
- `trace_context`: distributed trace propagation metadata.
- `payload`: versioned domain facts.

Define the producer, consumers, routing, ordering scope, retry/dead-letter policy,
and compatibility expectations. Consumers tolerate additional unknown fields and
must be safe against duplicate delivery. Do not assume exactly-once delivery or
global ordering.

Events describe facts that already occurred. A consumer does not mutate the
producer's database; it updates its own state or invokes an explicit contract.

## Idempotency

Follow [ADR-006](../adr/006-error-model-and-idempotency.md):

- Repeating an `Idempotency-Key` with the same request fingerprint returns the
  original response and status during the retention period.
- Reusing the same key with a different payload returns `409` with
  `IDEMPOTENCY_KEY_REUSED`.
- Store enough outcome data to distinguish replay from conflicting reuse.
- Event consumers record or derive a stable processing identity so duplicate
  delivery does not repeat business side effects.

Idempotency scope, retention, and fingerprint inputs are contract semantics.
Changing them requires the same compatibility review as request/response fields.

## Contract Change Checklist

Before merging a contract change, confirm:

- The owning service and source definition are clear.
- The change is additive or has an explicit versioned migration.
- Local generation succeeds, and affected generated consumers compile/test
  with a clean generated-output diff.
- All known consumers were inspected.
- Error, idempotency, authorization, and routing semantics are defined.
- Producer/consumer or black-box contract tests cover the changed boundary.
