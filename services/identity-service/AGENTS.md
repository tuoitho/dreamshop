# Identity Service Instructions

These instructions apply to `services/identity-service/` in addition to the root
[`AGENTS.md`](../../AGENTS.md).

## Ownership

This service owns:

- Credentials, registration, login, password verification, and optional
  `SUPER_ADMIN` bootstrap.
- User identity, profile, shipping addresses, administrative user management,
  and profile-location validation data.
- RBAC item/hierarchy storage, administrative RBAC APIs, and private gRPC
  authorization evaluation (`AuthorizationService`).
- Refresh-token sessions, rotation, revocation, and reuse detection.
- JWT issuance, signing-key metadata, and JWKS publication.

This service does not own:

- Catalog products or pricing.
- Inventory stock or reservations.
- Cart state or line items.
- Order lifecycle or state.
- The business meaning or resource rules of commerce permissions. Identity
  evaluates current RBAC assignments and hierarchy decisions for authorization,
  while the owning commerce service defines what each permission means and
  which resources it governs.

Other services consume Identity REST APIs, JWT/JWKS, or trusted claims. They do
not read Identity tables, reuse its repositories/entities, or modify identity
state directly.

## Security Boundaries

- Keep passwords one-way hashed; never log credentials or raw tokens.
- Keep private signing keys and populated secret files out of source control.
- Local keys under `deployments/secrets/identity/` are development-only.
- Preserve short-lived access tokens and HttpOnly refresh-cookie semantics.
- Preserve refresh-token rotation, session-family revocation on detected reuse,
  and explicit logout revocation behavior.
- Validate issuer, audience, key ID, algorithm, expiration, and caller context at
  the appropriate boundary.
- Treat `X-User-Id` and `X-User-Roles` as trusted only behind the authenticated
  edge/internal boundary that establishes them. Do not expose a public bypass.
- Return client-safe error envelopes and keep diagnostic detail in correlated
  logs.

Changing token claims, cookie attributes, TTLs, JWKS behavior, or session reuse
semantics is a contract/security change and requires consumer and rollout impact
analysis.

## Current Java Structure

- `auth/`: registration, login, refresh, logout, session domain/application/API,
  and persistence.
- `user/`: profile (`user/profile/`) and administrative user management
  (`user/admin/`) domain/application/API and persistence.
- `address/`: user shipping-address CRUD domain/application/API and persistence.
- `location/`: profile location reference data and API.
- `rbac/`: RBAC hierarchy/assignment domain, administrative REST API, and
  private `AuthorizationService` gRPC server (`GrpcServerLifecycle`).
- `security/`: JWT, JWKS, Argon2id password encoding, Spring Security, and
  caller context.
- `common/`: response/error, configuration, logging, and trace correlation.
- `src/main/resources/db/migration/`: Identity-owned Flyway migrations.

Keep domain and application decisions out of controllers, filters, JPA entities,
and provider/framework APIs. Controllers validate/map REST, application services
coordinate use cases, domain types enforce business rules, and persistence
adapters translate service-owned state.

Use Java 21 and Lombok for boilerplate. Follow `camelCase` in Java/REST and
`snake_case` for database columns. Do not expose JPA entities as API or contract
types.

## Persistence

- Change the schema only through a new ordered Flyway migration.
- Never edit an applied migration.
- Keep Identity tables, constraints, and migrations private to this service.
- Use database constraints for invariants that must survive concurrent writes.
- Validate migrations and repository behavior against PostgreSQL with
  Testcontainers when real database semantics matter.

## REST and Contracts

Identity public REST routes go through Kong. Preserve the repository success,
pagination, and error envelopes, stable error codes, `traceId`, and meaningful
mutation messages. REST/JSON fields use `camelCase`.

Use generated protobuf contracts only when Identity participates in an approved
internal gRPC boundary. Do not duplicate public identity/profile schemas in
another service for convenience.

Identity's private authorization gRPC listener binds `127.0.0.1:50054` locally
(`0.0.0.0:50054` on the isolated container network) and has no Kong or public
route. Actuator health is isolated on management port `8090`.

## Verification

During Red/Green, run the changed method from this service directory:

```powershell
mvn test '-Dtest=ProfilePatchTest#keepPreservesCurrentValue'
```

Then run its class:

```powershell
mvn test -Dtest=ProfilePatchTest
```

Finally run the affected reactor from repository root:

```powershell
mvn -pl services/identity-service -am test
```

Prefer plain JUnit 5/Mockito for domain and application logic, focused Spring
slices for HTTP/security/serialization, and Testcontainers for real persistence
or migration behavior. Add regression coverage for every changed security,
session, profile, or error path.

## Related Steering

- [Architecture](../../docs/steering/architecture.md)
- [Service boundaries](../../docs/steering/service-boundaries.md)
- [API contracts](../../docs/steering/api-contracts.md)
- [Change impact](../../docs/steering/change-impact.md)
- [Testing](../../docs/steering/testing.md)
- [Delivery](../../docs/steering/delivery.md)
