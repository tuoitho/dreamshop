# Monorepo Steering Documentation Design

## Context

The repository contains five independently deployable backend services, a React
frontend, shared API contracts, and deployment configuration. The root
`AGENTS.md` currently combines repository structure, commands, conventions,
REST response rules, and testing guidance in one file. It does not provide an
explicit reading path for architecture boundaries, contract changes, impact
analysis, or service-local instructions.

The steering documentation must reinforce that the monorepo is a source-code
organization model, not permission to weaken microservice boundaries. It must
also stay concise enough that coding agents can load only the instructions
relevant to the files they are changing.

## Goals

- Establish one concise root instruction file for rules that apply everywhere.
- Split detailed cross-repository guidance into focused steering documents.
- Add local instructions for each backend service and the frontend after the
  repository-wide rules are stable.
- Use progressive disclosure so agents can select relevant guidance without
  loading every document for every task.
- Preserve the repository's accepted architecture decisions, technology choices,
  response envelopes, naming conventions, and progressive testing workflow.
- Make change impact and compatibility analysis explicit before implementation.

## Non-goals

- Change service ownership, runtime architecture, or public API behavior.
- Introduce new services, shared packages, infrastructure, or build tooling.
- Duplicate ADR content or replace ADRs as records of architectural decisions.
- Document planned domains as though they already exist.
- Add local instructions outside the five current services and the frontend.

## Documentation Architecture

The instruction hierarchy will be:

```text
AGENTS.md
docs/steering/
  README.md
  architecture.md
  service-boundaries.md
  api-contracts.md
  change-impact.md
  testing.md
  delivery.md
services/
  bff-service/AGENTS.md
  catalog-service/AGENTS.md
  identity-service/AGENTS.md
  inventory-service/AGENTS.md
  order-service/AGENTS.md
ecommerce-fe/AGENTS.md
```

The root `AGENTS.md` is the entry point and contains only rules that must always
be visible: repository layout, core service-boundary constraints, repository-wide
commands, naming conventions, REST envelopes, required pre-change questions,
and links to focused steering documents.

`docs/steering/README.md` is the routing index. It maps common change types to
the smallest set of documents an agent must read. The remaining steering files
are canonical by topic. Local `AGENTS.md` files add ownership, boundaries,
technology conventions, directory guidance, and verification commands for the
nearest application surface.

## Progressive Disclosure Rules

Every task starts with the root `AGENTS.md`. The routing index then selects
additional guidance:

- Read `architecture.md` and `service-boundaries.md` for ownership changes,
  service interactions, shared code, persistence, or cross-service workflows.
- Read `api-contracts.md` for REST, OpenAPI, protobuf, gRPC, event, error-model,
  or idempotency changes.
- Read `change-impact.md` when a change could affect more than one service,
  a contract, persistence, configuration, or deployment.
- Read `testing.md` when selecting test scope or validating a bug fix, contract,
  integration, or end-to-end flow.
- Read `delivery.md` for migrations, environment variables, secrets, deployment
  manifests, rollout safety, or operational verification.
- Apply the nearest local `AGENTS.md` whenever work is performed inside a service
  or the frontend.

Local instructions may make repository rules more specific but may not relax
service boundaries, compatibility requirements, security constraints, or
verification expectations. When instructions appear to conflict, the more local
instruction governs only where it is compatible with repository-wide rules and
accepted ADRs. ADRs remain authoritative for architectural decisions; steering
documents translate those decisions into operational instructions.

## Repository-wide Documents

### Root `AGENTS.md`

The root document will retain verified repository facts and concise mandatory
rules:

- Five current backend services and their languages.
- Frontend, contracts, and deployment locations.
- The rule that services do not import another service's implementation or
  access another service's database.
- Approved communication paths: direct private gRPC, RabbitMQ events, and public
  REST routing through Kong, BFF, or approved transcoding as defined by ADR-008.
- The existing REST success, pagination, and error envelopes.
- Naming conventions across REST, protobuf, Java, TypeScript, Go, and databases.
- Common build, test, contract, frontend, and local infrastructure commands.
- A short pre-change checklist and links into `docs/steering/README.md`.

Detailed rationale and topic-specific procedures will move to canonical steering
documents instead of being repeated in the root file.

### Architecture

`architecture.md` will state the ownership defined by ADR-001:

- BFF owns client-specific commerce composition and REST adaptation.
- Identity owns credentials, profiles, roles, sessions, refresh tokens, and JWT
  issuance.
- Catalog owns product data and catalog caching.
- Order owns order lifecycle, order idempotency, and transactional outbox state.
- Inventory owns stock, reservations, and inventory event consumption.

It will define database ownership, service autonomy, and permitted shared code.
Domain rules remain inside their owning service; shared contract modules and
cross-cutting infrastructure must not become a path for domain coupling.

### Service Boundaries

`service-boundaries.md` will prohibit cross-service source imports, database
queries, foreign keys, runtime joins, and distributed database transactions. It
will guide the choice between direct gRPC and asynchronous events, permit small
read-model duplication, and require idempotency, retries, and compensating
actions for multi-service workflows. It will also preserve ADR-008's rule that
internal service traffic does not route through Kong or the BFF.

### API and Event Contracts

`api-contracts.md` will map the repository's actual contract layout:

- Protobuf sources under `contracts/proto/`.
- Generated Java and Go modules under `contracts/java/` and `contracts/go/`.
- REST/OpenAPI definitions under `contracts/openapi.yaml` where applicable.

The document will require additive changes by default, explicit versioning or
migration for breaking changes, tolerant consumers, generated-code regeneration,
and validation with `buf lint`. It will preserve the REST envelope, gRPC error
reason, trace correlation, and idempotency semantics established by repository
instructions and ADR-006. Event guidance will require stable identity, type,
version, occurrence time, correlation metadata, and duplicate-safe consumers.

### Change Impact

`change-impact.md` will require an impact inventory before implementation:

- `DIRECT`: files required to implement the behavior.
- `CONTRACT`: protobuf, OpenAPI, events, public DTOs, and compatibility surfaces.
- `DEPENDENT`: consumers of changed contracts.
- `INFRA`: migrations, manifests, environment variables, secrets, and resources.
- `TEST`: verification required for changed behavior and compatibility.

Dependent services are inspected for impact but changed only when the contract
requires it. A change that can remain inside one service must remain there.

### Testing

`testing.md` will preserve the existing smallest-sufficient-scope principle and
Java progressive Maven sequence. It will add guidance for unit, focused framework
slice, repository/integration, contract, and critical end-to-end tests. Go and
frontend commands will remain aligned with current repository tooling. Bug fixes
must begin with a reproducing regression test, followed by the smallest fix and
progressively broader verification.

### Delivery

`delivery.md` will cover migration ownership, backward-compatible rollout order,
configuration and secret handling, deployment manifest impact, observability,
and final verification evidence. Infrastructure runs in Docker Compose while
application services run directly during local development. Kong-to-host
upstreams use `host.docker.internal:<port>` as already established.

## Local Instructions

Local `AGENTS.md` files will be added only after the repository-wide documents
are established. Each local file will contain:

- What the application surface owns and explicitly does not own.
- Allowed inbound and outbound dependencies.
- Service-specific architecture and directory conventions that exist in the
  workspace.
- The smallest useful test and verification commands.
- Links to only the relevant repository steering topics.

The service instructions will describe current capabilities rather than planned
payment, notification, cart, review, or search services. The frontend document
will cover client presentation, routing, session use, API adapters, UI validation,
Vitest and Testing Library, linting, and production builds. It will not assign
backend domain ownership to the frontend or allow client-side state to become
the source of truth for server-owned business rules.

## Delivery Phases

### Phase 1: Repository-wide steering

1. Refactor root `AGENTS.md` into a concise mandatory entry point.
2. Add the `docs/steering/` routing index and six canonical topic documents.
3. Validate links, repository paths, commands, terminology, and consistency with
   ADR-001, ADR-006, and ADR-008.

### Phase 2: Application-local steering

1. Inspect each service and the frontend for existing structure and commands.
2. Add a scoped `AGENTS.md` to each of the five services and the frontend.
3. Check that local rules specialize without duplicating or weakening root rules.
4. Validate that ownership statements describe the implemented or accepted
   architecture and do not invent future services.

## Validation

Documentation validation will include:

- Confirming every referenced file and command exists or is intentionally a
  documented convention.
- Searching for contradictory ownership, routing, naming, compatibility, and
  response-envelope rules.
- Checking all Markdown links from the root and routing index.
- Confirming the root file remains concise and detailed rules have one canonical
  location.
- Reviewing local instructions against service code, build files, and accepted
  ADRs before they are added.
- Inspecting the final diff to ensure no unrelated files changed and no
  conversation-specific commentary leaked into the artifacts.

