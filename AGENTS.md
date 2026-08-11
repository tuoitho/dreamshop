# Repository Guidelines

## Project Structure & Module Organization

Application services live in `services/`: Identity and Order use Java/Spring Boot, while BFF, Catalog, and Inventory use Go. The React/Vite client is in `ecommerce-fe/`. Protobuf definitions and generated modules are in `contracts/`. Infrastructure configuration is under `deployments/`.

Keep production code beside its service. Java tests belong in `src/test/java`; Go tests use `_test.go` files next to their packages; frontend tests use `*.test.ts` or `*.test.tsx` under `ecommerce-fe/src`.

## Build, Test, and Development Commands
- `docker compose -f deployments/docker/compose.foundation.yml up -d` starts PostgreSQL, Redis, RabbitMQ, and Kong.
- `.\services\identity-service\start-local.ps1` or `.\services\order-service\start-local.ps1` runs a Java service on Windows.
- `mvn test` builds and tests all Maven modules.
- `go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...` runs all Go tests.
- `npm --prefix ecommerce-fe run dev` starts the frontend; replace `dev` with `test`, `lint`, or `build` for the corresponding check.
- `buf lint` validates protobuf contracts; `buf generate` regenerates clients.

Run infrastructure in Docker Compose, but run application services directly from a Windows terminal or IDE. When containerized Kong proxies to a Windows-hosted service, use `host.docker.internal:<port>` as the upstream.

## Coding Style & Naming Conventions
Use Java 21, lombok for boilerplate reduction.
Use the naming convention native to each protocol and application layer:

- REST/JSON fields use `camelCase`
- Protobuf fields in `.proto` files use `lower_snake_case`. Generated clients convert these names to the conventions of their target languages.
- Java and TypeScript fields use `camelCase`.
- Go identifiers use `PascalCase` when exported and `camelCase` when unexported.
- Database columns use `snake_case`.

## REST Response Contract

Use one response envelope for REST/JSON endpoints.

- Successful response: `{ "data": <payload>, "message": "..." }`.
- Successful response without a payload: use `"data": null` and still provide `message`.
- Paginated response: put pagination metadata inside `data`, using `{ "items": [], "page": 1, "pageSize": 20, "totalItems": 0, "totalPages": 0 }`.
- Error response: `{ "code": "STABLE_ERROR_CODE", "message": "...", "errors": {}, "traceId": "..." }`.
- Omit `errors` when there are no field-level or structured details; do not return an ad-hoc error shape.
- Business errors use the same error envelope and an appropriate non-2xx HTTP status. Keep `code` stable for client behavior and use `message` for human-readable display.
- Mutation endpoints must provide a meaningful success `message`; clients may use it for success notifications.

## Testing Guidelines

Java tests use JUnit 5, Spring test support, and Testcontainers where database behavior matters. Frontend tests use Vitest and Testing Library. Name tests after the subject and behavior, and add regression coverage for every changed path. No repository-wide coverage threshold is enforced; meaningful behavior coverage is expected.

### Java/Maven Progressive Testing

Keep the Red-Green-Refactor feedback loop narrow, then expand verification progressively:

1. During Red/Green, run only the newly added or modified test method from the affected service directory: `mvn test '-Dtest=ProfilePatchTest#keepPreservesCurrentValue'`.
2. After that method passes, run its test class: `mvn test -Dtest=ProfilePatchTest`.
3. After the implementation is stable, run the affected module and its required reactor dependencies from the repository root: `mvn -pl services/identity-service -am test`.

Do not run the full Maven suite after every small edit. If the final suite cannot run because of an environmental or external dependency blocker, report the exact blocker and which narrower checks passed; do not claim full verification.

Prefer the smallest Spring test scope that proves the behavior:

- Test domain and service logic with plain JUnit 5 and Mockito; do not start Spring for logic that can be tested directly.
- Use focused slices such as `@WebMvcTest`, `@DataJpaTest`, or `@JsonTest` for framework boundaries.
- Reserve `@SpringBootTest` and Testcontainers for integration behavior that genuinely needs the application context or real infrastructure.

Keep Maven output out of the agent context unless it is relevant. Capture output to a log, inspect the summary and failing tests first, then read only the necessary stack-trace section. In PowerShell, for example:

```powershell
$testLog = Join-Path $env:TEMP 'identity-service-test.log'
mvn test -Dtest=ProfilePatchTest *> $testLog
Select-String -Path $testLog -Pattern 'BUILD (SUCCESS|FAILURE)|Tests run:|Failures:|Errors:'
Get-Content $testLog -Tail 80
```

Use `mvn -q` only when its reduced output still preserves enough failure information for diagnosis.
