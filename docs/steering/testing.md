# Testing Steering

Use the smallest test scope that provides sufficient confidence, then expand
verification in proportion to contract, persistence, infrastructure, and
cross-service risk.

## Test Scope Selection

### Unit tests

Use unit tests for domain rules, validation, calculations, state transitions,
idempotency decisions, and error mapping. Unit tests must not require databases,
message brokers, network calls, or an application context.

### Focused framework tests

Use focused boundaries when framework behavior is relevant:

- `@WebMvcTest` for Spring HTTP/security mapping.
- `@DataJpaTest` for JPA repository behavior that does not require the full app.
- `@JsonTest` for serialization contracts.
- Go package tests for handlers, config, and adapters.
- Vitest with Testing Library for user-visible frontend behavior.

Do not start Spring for plain application or domain logic.

### Integration tests

Use integration tests for database repositories/migrations, RabbitMQ behavior,
external API adapters, gRPC/REST boundaries, security wiring, and configuration
that depends on a real runtime. Use Testcontainers when real infrastructure
semantics matter. Keep each test responsible for one integration boundary.

### Contract tests

Validate producer and consumer compatibility independently from a full
end-to-end environment. Contract tests cover protobuf evolution, REST envelopes,
HTTP/gRPC error mapping, event versions, required metadata, and tolerant
consumption of additive fields.

### End-to-end tests

Reserve E2E tests for critical cross-service flows and edge routing, such as an
authenticated client request through Kong and a commerce workflow spanning
Order and Inventory. Do not test every validation branch through E2E; keep edge
cases at the narrowest reliable layer.

## Bug Fixes

For a reproducible bug:

1. Add or identify a test that fails for the reported behavior.
2. Run it and confirm the expected failure reason.
3. Implement the smallest owning-surface fix.
4. Run the focused test until it passes.
5. Expand to its class/package/module and affected consumers.

Do not delete or weaken a valid test merely to make a build pass. Remove a test
only when investigation establishes that it asserts behavior the system does not
own or require.

## Java/Maven Progressive Testing

Keep the Red-Green-Refactor loop narrow:

1. From the affected service directory, run the changed method:

   ```powershell
   mvn test '-Dtest=ProfilePatchTest#keepPreservesCurrentValue'
   ```

2. Run its test class:

   ```powershell
   mvn test -Dtest=ProfilePatchTest
   ```

3. From repository root, run the affected module and reactor dependencies:

   ```powershell
   mvn -pl services/identity-service -am test
   mvn -pl services/order-service -am test
   ```

Prefer plain JUnit 5 and Mockito for domain/application logic, focused Spring
slices for framework boundaries, and `@SpringBootTest` plus Testcontainers only
when the application context or real infrastructure is required.

Keep verbose Maven output out of review context. Capture it, inspect summaries,
then read only the relevant failure section:

```powershell
$testLog = Join-Path $env:TEMP 'identity-service-test.log'
mvn test -Dtest=ProfilePatchTest *> $testLog
Select-String -Path $testLog -Pattern 'BUILD (SUCCESS|FAILURE)|Tests run:|Failures:|Errors:'
Get-Content $testLog -Tail 80
```

Use `mvn -q` only when reduced output still preserves enough diagnostic detail.

## Go Testing

From an individual Go service:

```powershell
go test ./...
```

From repository root (on Linux/CI; on Windows PowerShell replace
`./services/cart-service/...` with `./services/cart-service/internal/...`
because `cart-service/cmd/server` tests use Unix `syscall.Kill`):

```powershell
go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/internal/...
```

Place `_test.go` files beside the package under test. Prefer table-driven tests
when several inputs share the same behavior. Use fakes at service boundaries;
use real infrastructure only for adapter behavior that depends on it.

## Frontend Testing

Use Vitest and Testing Library. Assert outcomes users observe rather than
component internals. Keep tests under `ecommerce-fe/src` using `*.test.ts` or
`*.test.tsx`.

Run:

```powershell
npm --prefix ecommerce-fe run test
npm --prefix ecommerce-fe run lint
npm --prefix ecommerce-fe run build
```

Test API adapters for envelopes and stable error codes, auth/session behavior for
refresh and retry constraints, protected routes, validation feedback, and
mutation success notifications.

## Verification Handoff

Report exact commands and outcomes. If an external dependency blocks broad
verification, report the dependency, failure, and narrower checks that passed.
Do not claim a module, build, or suite passed unless the corresponding command
completed successfully in the current change state.
