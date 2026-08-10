# Repository Guidelines

## Project Structure & Module Organization

Application services live in `services/`: Identity and Order use Java/Spring Boot, while BFF, Catalog, and Inventory use Go. The React/Vite client is in `ecommerce-fe/`. Protobuf definitions and generated modules are in `contracts/`. Infrastructure configuration is under `deployments/`, operational checks are in `scripts/`.

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

Use Java 21 conventions: four-space indentation, `PascalCase` types, and `camelCase` members. Format Go with `gofmt`; keep package names short and lowercase. Use two spaces in TypeScript/TSX, `PascalCase` React components, and `camelCase` hooks and utilities. Keep YAML indentation at two spaces. Never edit generated protobuf output manually.

## Testing Guidelines

Java tests use JUnit 5, Spring test support, and Testcontainers where database behavior matters. Frontend tests use Vitest and Testing Library. Name tests after the subject and behavior, and add regression coverage for every changed path. No repository-wide coverage threshold is enforced; meaningful behavior coverage is expected.