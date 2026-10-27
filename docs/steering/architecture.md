# Architecture Steering

The repository follows the service ownership and isolation decisions in
[ADR-001](../adr/001-service-boundaries.md),
[ADR-009](../adr/009-cart-service.md), and
[ADR-010](../adr/010-product-favorites-in-cart-service.md). Public routing follows
[ADR-008](../adr/008-hybrid-rest-bff-grpc-transcoding.md).

## Capability Ownership

Each business capability has one owning service:

- **BFF** owns client-specific commerce composition, REST adaptation, custom
  error mapping, and orchestration required by the frontend.
- **Identity** owns credentials, registration and login, profiles, shipping
  addresses, location reference data, RBAC hierarchy and authorization
  evaluation, refresh-token sessions, JWT issuance, and JWKS publication.
- **Catalog** owns product/catalog data, soft-delete state, and catalog caching.
- **Cart** owns authenticated user shopping carts, persisted line items,
  saved product favorites, and enriched cart/favorite projections assembled over
  private gRPC.
- **Order** owns the order lifecycle, order state, command idempotency, and
  transactional outbox state.
- **Inventory** owns stock, reservation lifecycle, availability invariants, and
  inventory event consumption.

Consumers use the owning service's API or events. They do not reimplement its
rules merely because the code is available in the same repository.

The frontend owns presentation, client routing, interaction state, API
adaptation, and session user experience. It is not the source of truth for
server-owned business rules.

## Database Ownership

Each service owns its persistence model, schema, migrations, and transactional
invariants.

Forbidden across service boundaries:

- Reading or writing another service's tables or schema.
- Creating cross-service foreign keys, views, triggers, or stored procedures.
- Reusing another service's repository or persistence entity.
- Coordinating one database transaction across multiple services.

Required alternatives include private gRPC APIs, asynchronous events, and
service-owned read models. A service may expose facts from its data; ownership
of the underlying data and write decisions remains with that service.

## Service Autonomy

Every service must remain:

- Independently testable.
- Independently deployable.
- Independently scalable according to its workload.
- Independently observable through health, logs, metrics, and traces.

A change in one service must not require rebuilding every service unless a
shared contract changes. Consumers of a changed contract are inspected for
compatibility, but they are modified only when the contract requires it.

Local development may share PostgreSQL, Redis, RabbitMQ, or a Docker network.
Shared infrastructure does not imply shared data ownership or a shared runtime
lifecycle.

## Shared Code

Shared code is appropriate for:

- Protobuf sources and generated Java/Go contract modules.
- Logging, tracing, metrics, and health conventions.
- Generic configuration loading and security middleware.
- Infrastructure clients with no service-owned business semantics.

Shared code must not contain:

- Order state-transition rules.
- Inventory reservation or availability rules.
- Identity credential, session, profile, or RBAC rules.
- Catalog product rules.
- Cart line-item or enrichment rules.
- Client-specific workflow composition that belongs in BFF.

Do not create a shared abstraction only to remove a small amount of duplication.
Duplication is preferable to hidden coupling when the duplicated code may evolve
with different bounded contexts. When ownership is uncertain, keep the code in
the service that uses the business rule and resolve ownership before sharing it.

## Adding or Splitting Capabilities

Do not add a service solely to demonstrate a technology or because a package has
grown. A new boundary requires clear ownership of data, invariants, APIs/events,
failure handling, deployment, and observability. Record a material change to the
accepted service map in an ADR before implementation.
