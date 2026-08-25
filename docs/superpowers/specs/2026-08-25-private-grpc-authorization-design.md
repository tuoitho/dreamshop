# Private gRPC Authorization Design

## Summary

Identity Service exposes a private gRPC authorization API that evaluates the
current RBAC graph in `identity_db`. Domain services use the API for permission
decisions without reading Identity tables, importing Identity implementation
code, or relying on permission claims embedded in an access token.

The first protected operation is adding a product to a cart. Cart Service asks
Identity whether the authenticated user has `cart.item.add` after validating the
request and before performing any persistent mutation. Identity reads current
RBAC state for every request, so grants, revocations, account deactivation, and
hierarchy changes affect the next operation without waiting for JWT expiry.

## Goals

- Preserve Identity as the sole owner of users, account state, RBAC items,
  hierarchy, and assignments.
- Preserve Cart as the owner of cart operations and the meaning of
  `cart.item.add`.
- Keep JWTs responsible for authentication and caller identity, not current
  permission state.
- Make authorization denials distinguishable from authorization dependency
  failures at every boundary.
- Support efficient batch permission checks for future domain-service use.
- Fail closed without retrying authorization in the request path.

## Scope

### Included

- A versioned `ecommerce.identity.v1.AuthorizationService` protobuf contract.
- Generated Java and Go contract artifacts.
- A private Identity gRPC server on port `50054`.
- Current-state single and batch RBAC queries in Identity.
- Provisioning `cart.item.add` and granting it to the built-in `CUSTOMER` role.
- A Cart-owned Go authorization client with a bounded per-call timeout.
- An authorization guard on `CartService.AddItem` before cart mutation.
- Stable Cart gRPC errors and BFF mappings for denial and dependency failure.
- Configuration, topology, operation, contract, and regression-test updates.

### Excluded

- Applying permissions to `GetCart`, `UpdateItemQuantity`, `RemoveItem`, or
  `ClearCart`.
- Adding permissions to JWTs.
- Caching authorization decisions in Identity or consumers.
- A shared business-level authorization client implementation.
- Direct RBAC table access or Identity source imports from another service.
- Public exposure of the authorization API through Kong or BFF.
- Service identity, mTLS, or a general internal zero-trust transport layer.
- Order authorization or an order-cancellation workflow.

## Ownership and Boundaries

Identity owns the representation and evaluation of RBAC assignments and
hierarchy. Its authorization API answers whether a user currently inherits a
named permission. It does not decide whether a cart mutation is valid or define
which Cart operation requires that permission.

Cart owns the permission name `cart.item.add`, where in the operation it is
enforced, and all resource and business checks for adding a cart item. The
authenticated user ID continues to select the user's own cart; the client cannot
provide a different cart owner in the public request body.

BFF remains a transport adapter. It forwards the caller identity established by
Kong, invokes Cart through the generated Cart contract, and maps stable gRPC
outcomes to the public REST error envelope. BFF does not evaluate RBAC itself.

The contracts modules contain protobuf definitions and generated transport
types only. Every consuming service owns its adapter, timeout policy, domain
error mapping, and enforcement point.

## Alternatives

### Cart calls Identity directly

This is the selected approach. The service that owns the protected operation
also enforces its permission, so internal callers cannot bypass authorization by
skipping BFF. The additional synchronous dependency is bounded by a short
timeout and fail-closed behavior.

### BFF calls Identity before Cart

This reduces Cart wiring but makes authorization dependent on one ingress path.
Another internal Cart caller could bypass the check, and each new adapter would
have to reproduce enforcement. This approach is not used.

### Permissions are embedded in JWTs

This avoids a synchronous call but leaves revoked permissions valid until token
expiry and makes token size grow with the permission set. It does not meet the
current-state requirement and is not used.

## Request Flow

```text
Client
  -> Kong validates JWT and establishes X-User-Id
  -> BFF sends AddItemRequest(user_id, product_id, variant_label, quantity)
  -> Cart validates required fields and quantity
  -> Cart calls Identity CheckPermission(user_id, "cart.item.add")
       -> Identity evaluates active user + assignments + RBAC hierarchy
  -> allowed=true: Cart continues with EnsureCart and AddItem
  -> allowed=false: Cart returns PERMISSION_DENIED
  -> Identity failure/timeout: Cart returns authorization UNAVAILABLE
  -> BFF maps the stable outcome to the REST error envelope
```

Authorization runs before `EnsureCart`, `AddItem`, Catalog calls, or Inventory
calls. A denial or dependency failure therefore causes no Cart-owned write and
no unnecessary downstream work.

## Protobuf Contract

The source contract lives in a dedicated
`contracts/proto/identity/v1/authorization.proto` file while retaining the
existing `ecommerce.identity.v1` package and language package options.

```proto
syntax = "proto3";

package ecommerce.identity.v1;

option go_package = "ecommerce.local/contracts/go/identity/v1;identityv1";
option java_multiple_files = true;
option java_package = "com.ecommerce.contracts.identity.v1";

service AuthorizationService {
  rpc CheckPermission(CheckPermissionRequest)
      returns (CheckPermissionResponse);
  rpc CheckPermissions(CheckPermissionsRequest)
      returns (CheckPermissionsResponse);
}

message CheckPermissionRequest {
  string user_id = 1;
  string permission = 2;
}

message CheckPermissionResponse {
  bool allowed = 1;
}

message CheckPermissionsRequest {
  string user_id = 1;
  repeated string permissions = 2;
}

message CheckPermissionsResponse {
  repeated PermissionDecision decisions = 1;
}

message PermissionDecision {
  string permission = 1;
  bool allowed = 2;
}
```

Permission identifiers remain strings because bounded contexts can add stable,
namespaced permissions without coordinating a global enum release. The batch
response contains exactly one decision for each requested permission in input
order, including duplicate inputs. Consumers do not infer authorization from a
missing result.

## Identity Authorization Provider

### Input rules

- `user_id` must be a syntactically valid UUID.
- A single permission must be non-blank and must not contain leading or trailing
  whitespace.
- A batch must contain between 1 and 100 permissions, and every item follows the
  single-permission rule.
- Invalid input returns gRPC `INVALID_ARGUMENT`; it is not an authorization
  denial.

### Decision rules

- An active user is allowed when the requested permission is reachable from any
  assigned role through the current RBAC hierarchy.
- `SUPER_ADMIN` retains its existing implicit access to every current and future
  permission.
- A missing or inactive user is denied.
- A missing, unassigned, or unreachable permission is denied.
- A valid request returns a normal response even when every decision is false.
- Unexpected persistence failures return gRPC `INTERNAL` without exposing SQL or
  internal exception details.

The existing authorization application service gains UUID-based single and
batch operations. Its Spring Security-facing operation continues to delegate to
the same decision logic so Identity REST checks and private gRPC checks do not
diverge.

Single checks read current database state once. Batch checks use one recursive
RBAC query and construct ordered decisions from its result; they do not issue one
database query per permission. No result is cached in process, Redis, or a JWT.

### Permission provisioning

A new immutable Flyway migration registers:

- `cart.item.add`, type `PERMISSION`, as a protected system item;
- a `CUSTOMER -> cart.item.add` hierarchy edge.

Administrators may remove or restore the hierarchy edge through existing RBAC
management behavior. The permission name itself cannot be renamed or deleted,
because it is a durable cross-service identifier. It is not added to Identity's
Java permission constants: Identity stores and evaluates the name, while Cart
owns its operational meaning.

## Identity gRPC Runtime

Identity hosts HTTP and private gRPC in the same application process with
separate listeners. The canonical private gRPC port is `50054`. Local
development binds it to `127.0.0.1`; container or deployed configuration may
bind it to an internal interface. No Kong route exposes this listener.

The server registers the generated `AuthorizationService` base and delegates to
the Identity application layer. Graceful application shutdown stops accepting
new RPCs and allows bounded in-flight work to finish.

The existing internal gRPC transport convention is plaintext. Network policy
and listener exposure are the security boundary for this scope. Production
service authentication and mTLS require a separate design applied consistently
to all internal services.

## Cart Authorization Client

Cart adds an `internal/authorization` adapter around the generated Go client.
The adapter owns its connection, applies the configured timeout to every call,
propagates the parent context, and closes the connection during service
shutdown.

Configuration adds:

- `IDENTITY_GRPC_ADDR`, default `127.0.0.1:50054`;
- `IDENTITY_GRPC_TIMEOUT`, default `500ms` and required to be positive.

The adapter exposes Cart-oriented results rather than Identity persistence or
domain types. `AddItem` uses the constant `cart.item.add` owned by Cart. It does
not retry because an additional attempt increases request latency and cannot
make an unknown authorization result safe.

Cart startup constructs the authorization client beside its existing Catalog
and Inventory clients and includes it in deterministic cleanup. A configured
endpoint need not be reachable during startup because gRPC connections are
lazy; request-time checks remain fail-closed.

## Error Semantics

Identity returns decisions as data. Cart converts those decisions and dependency
failures into errors owned by the protected Cart operation.

| Condition | Identity outcome | Cart gRPC outcome | Public HTTP outcome |
| --- | --- | --- | --- |
| Permission granted | `allowed=true` | Normal `AddItemResponse` | `200` |
| User inactive/missing or permission denied | `allowed=false` | `PERMISSION_DENIED` with reason `CART_PERMISSION_DENIED` | `403 FORBIDDEN` |
| Identity timeout or transport/server failure | gRPC error | `UNAVAILABLE` with reason `AUTHORIZATION_UNAVAILABLE` | `503 AUTHORIZATION_SERVICE_UNAVAILABLE` |
| BFF cannot reach Cart | No Cart response | Downstream `UNAVAILABLE` without the authorization reason | `502 CART_UNAVAILABLE` |

Cart attaches `google.rpc.ErrorInfo` using domain `ecommerce.cart.v1` so BFF
branches on a stable reason instead of human-readable text. BFF preserves the
standard error envelope and a client-safe `traceId`.

Parent request cancellation remains cancellation rather than being relabeled as
an authorization outage. Other Identity errors are fail-closed and do not fall
back to JWT roles or a cached decision.

## Observability and Health

Authorization logs identify the owning service, permission, outcome category,
and trace or correlation identifier without logging tokens or credentials.
Denials and dependency failures are distinct outcomes. Health endpoints remain
process/readiness signals rather than invoking Identity for every probe; a
request-time Identity outage is visible through authorization failure logs and
public `503` responses.

## Compatibility and Rollout

The protobuf addition is backward compatible. Existing Identity, Cart, and BFF
RPC methods and messages are unchanged. Generated code is committed for both
languages.

Deployment order is:

1. Publish the additive contract and deploy Identity with the Flyway migration
   and gRPC server.
2. Verify the Identity listener and a granted/denied authorization check.
3. Deploy Cart with the Identity client and `AddItem` guard.
4. Deploy BFF with stable `403` and `503` mappings.

Deploying Cart before Identity would deliberately fail closed and make add-item
requests unavailable, so the provider-first order is required. Rolling Cart
back removes the new check without requiring a database rollback. The additive
permission and hierarchy edge may remain in Identity during rollback.

## Change Impact

### DIRECT

- Identity RBAC application and persistence query behavior.
- Identity private gRPC server and runtime configuration.
- Cart authorization client, configuration, dependency wiring, and `AddItem`
  guard.
- BFF Cart error mapping.

### CONTRACT

- New `ecommerce.identity.v1.AuthorizationService` messages and RPCs.
- Stable Cart gRPC `ErrorInfo` reasons for authorization outcomes.
- Public BFF `403` and `503` error codes for the add-item route.
- New Identity and Cart gRPC configuration variables and port topology.

### DEPENDENT

- Generated Java and Go consumers compile against the additive contract.
- BFF consumes the changed Cart error semantics.
- Existing Cart clients remain wire-compatible because `AddItemRequest` and
  `AddItemResponse` do not change.

### INFRA

- Identity Flyway migration for the permission registry and default role edge.
- Identity private port `50054` and service discovery documentation.
- Cart Identity address and timeout configuration.

## Testing Strategy

### Contract

- Check protobuf formatting and linting.
- Generate Java and Go artifacts and confirm a clean regeneration diff.
- Build the generated Java module and compile affected Go modules.

### Identity

- Unit-test UUID-based single and batch authorization application operations.
- PostgreSQL integration-test active, inactive, missing, inherited,
  `SUPER_ADMIN`, unknown-permission, grant, and revocation decisions.
- Verify a revocation changes the next query result without process restart or
  token renewal.
- Verify a batch is evaluated by the batch repository operation and returns
  decisions in input order.
- Exercise the gRPC boundary through a real in-process server and generated
  client for success, denial, invalid input, batch limits, and internal-error
  mapping.
- Verify the application starts with the configured private gRPC listener.
- Verify the Flyway migration provisions the protected permission and
  `CUSTOMER` edge on both a fresh schema and an upgraded schema.

### Cart

- Exercise the authorization adapter against an in-process gRPC server to prove
  request mapping, timeout, response mapping, and connection cleanup.
- Verify valid `AddItem` calls authorization with the caller user ID and
  `cart.item.add`.
- Verify denial prevents `EnsureCart`, `AddItem`, Catalog, and Inventory calls.
- Verify Identity timeout/failure prevents those same side effects and returns
  `UNAVAILABLE` with `AUTHORIZATION_UNAVAILABLE`.
- Verify an allowed decision preserves existing add-item behavior.
- Test default, overridden, invalid, and non-positive Identity timeout config.
- Test production dependency construction and cleanup.

### BFF

- Verify Cart `PERMISSION_DENIED` with `CART_PERMISSION_DENIED` maps to the
  standard `403 FORBIDDEN` envelope.
- Verify Cart `UNAVAILABLE` with `AUTHORIZATION_UNAVAILABLE` maps to the standard
  `503 AUTHORIZATION_SERVICE_UNAVAILABLE` envelope.
- Verify an unclassified Cart connection failure remains `502 CART_UNAVAILABLE`.
- Preserve existing validation, authentication, success, and error tests.

### Progressive verification

Run focused Red-Green-Refactor checks first, then:

- `buf format --diff --exit-code`, `buf lint`, and `buf generate`;
- Identity Maven tests with reactor dependencies;
- Cart Go tests;
- BFF Go tests;
- the repository's affected Java and Go aggregate checks.

An environment-backed cross-service smoke test verifies that a customer can add
an item, removing the `CUSTOMER -> cart.item.add` edge causes the next request to
return `403`, restoring the edge restores access, and stopping Identity causes
the next request to return `503` without a Cart mutation.

## Acceptance Criteria

- A user currently inheriting `cart.item.add` can add a valid product variant to
  their own cart.
- Removing the permission edge or applicable role makes the next add-item call
  return `403` without waiting for JWT expiry.
- Regranting the permission makes the next valid call eligible to proceed.
- An inactive or missing user cannot add an item.
- Identity timeout or failure returns `503` and causes no Cart mutation.
- Invalid add-item requests are rejected before authorization or mutation.
- Other Cart operations retain their existing authorization behavior.
- No non-Identity service reads Identity tables or imports Identity
  implementation code.
- The authorization listener has no Kong or public BFF route.
- Generated contracts, affected service tests, and boundary tests pass.
