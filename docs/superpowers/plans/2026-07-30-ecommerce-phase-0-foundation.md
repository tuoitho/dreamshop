# E-commerce Phase 0 Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tạo monorepo chạy được local với 5 service skeleton, infrastructure Docker, Kong, protobuf contract và kiểm chứng tự động cho nền tảng.

**Architecture:** Identity và Order dùng Java/Spring Boot; BFF, Catalog, Inventory dùng Go. Client traffic đi qua Kong; tất cả service có HTTP health endpoint, còn service-to-service contract được quản lý ở một Buf module chung và generate cho Go/Java. Phase này chưa triển khai nghiệp vụ checkout, database entity hay RabbitMQ consumer.

**Tech Stack:** Java 21, Spring Boot + Actuator, Go, gRPC, Buf v2, PostgreSQL 16, Redis 7, RabbitMQ 3 Management, Kong DB-less, Docker Compose, PowerShell and Make.

---

## File structure after this phase

```text
.
├── .env.example
├── .gitignore
├── Makefile
├── README.md
├── go.work
├── buf.yaml
├── buf.gen.yaml
├── contracts/
│   ├── proto/{common,catalog,inventory,order,identity}/v1/*.proto
│   ├── go/{go.mod,**/*.pb.go}
│   └── java/{pom.xml,src/main/generated/**}
├── deployments/
│   ├── docker/compose.foundation.yml
│   ├── kong/kong.yaml
│   └── postgres/init/01-create-databases.sql
├── scripts/verify-foundation.ps1
└── services/
    ├── bff-service/
    ├── catalog-service/
    ├── inventory-service/
    ├── identity-service/
    └── order-service/
```

Generated contract output is reproducible build output. Commit `buf.lock`, source `.proto`, generation configuration and the two contract-module manifests; do not hand-edit or commit generated files.

### Task 1: Establish the repository contract and a failing foundation check

**Files:**

- Create: `.gitignore`
- Create: `.env.example`
- Create: `Makefile`
- Create: `go.work`
- Create: `scripts/verify-foundation.ps1`

- [ ] **Step 1: Write the foundation verifier before creating its required files**

```powershell
$ErrorActionPreference = 'Stop'
$required = @(
  'buf.yaml', 'buf.gen.yaml', 'deployments/docker/compose.foundation.yml',
  'deployments/kong/kong.yaml', 'contracts/proto/common/v1/common.proto',
  'services/identity-service/pom.xml', 'services/order-service/pom.xml',
  'services/bff-service/go.mod', 'services/catalog-service/go.mod',
  'services/inventory-service/go.mod'
)

$missing = $required | Where-Object { -not (Test-Path $_) }
if ($missing) { throw "Missing foundation files: $($missing -join ', ')" }
docker compose -f deployments/docker/compose.foundation.yml config --quiet
buf lint
Write-Host 'Foundation checks passed.'
```

- [ ] **Step 2: Run the verifier and confirm it fails for missing files**

Run: `pwsh -File scripts/verify-foundation.ps1`

Expected: failure listing missing foundation files.

- [ ] **Step 3: Create root safety and developer-entry files**

Initialize source control before the first commit:

```powershell
git init
```

Use these values in `.env.example`; do not add a real signing key or password to Git:

```dotenv
POSTGRES_USER=ecommerce
POSTGRES_PASSWORD=change-me-local-only
RABBITMQ_DEFAULT_USER=ecommerce
RABBITMQ_DEFAULT_PASS=change-me-local-only
IDENTITY_JWT_ISSUER=http://identity-service:8080
IDENTITY_JWT_AUDIENCE=ecommerce-api
```

Use this minimum `.gitignore`:

```gitignore
.env
.idea/
.vscode/
**/target/
**/tmp/
**/*.pem
**/*.key
contracts/go/**/*.pb.go
contracts/java/src/main/generated/
```

Create `go.work` with the three Go services:

```go
go 1.24

use (
    ./contracts/go
    ./services/bff-service
    ./services/catalog-service
    ./services/inventory-service
)
```

Create a Makefile with the stable entry points:

```makefile
.PHONY: up down logs proto lint verify

up:
	docker compose -f deployments/docker/compose.foundation.yml up -d --build

down:
	docker compose -f deployments/docker/compose.foundation.yml down

logs:
	docker compose -f deployments/docker/compose.foundation.yml logs -f

proto:
	buf generate

lint:
	buf lint

verify:
	pwsh -File scripts/verify-foundation.ps1
```

- [ ] **Step 4: Run the verifier again**

Run: `pwsh -File scripts/verify-foundation.ps1`

Expected: it still fails only for Compose, proto and service files not yet created.

- [ ] **Step 5: Commit the root contract**

```powershell
git add .gitignore .env.example Makefile go.work scripts/verify-foundation.ps1
git commit -m "chore: add foundation developer contract"
```

### Task 2: Provision local infrastructure with explicit ownership boundaries

**Files:**

- Create: `deployments/docker/compose.foundation.yml`
- Create: `deployments/postgres/init/01-create-databases.sql`
- Create: `deployments/kong/kong.yaml`

- [ ] **Step 1: Add a failing Compose configuration check to the verifier**

Keep `docker compose -f deployments/docker/compose.foundation.yml config --quiet` in `scripts/verify-foundation.ps1` before creating the Compose file.

- [ ] **Step 2: Verify the missing Compose file fails**

Run: `docker compose -f deployments/docker/compose.foundation.yml config --quiet`

Expected: non-zero exit because the file does not exist.

- [ ] **Step 3: Create PostgreSQL initialization and DB-less Kong configuration**

`deployments/postgres/init/01-create-databases.sql`:

```sql
CREATE DATABASE identity_db;
CREATE DATABASE order_db;
CREATE DATABASE catalog_db;
CREATE DATABASE inventory_db;
```

`deployments/kong/kong.yaml` must declare upstream service names only; it must not publish Catalog, Order or Inventory directly:

```yaml
_format_version: "3.0"
services:
  - name: identity-service
    url: http://identity-service:8080
    routes:
      - name: identity-public
        paths: ["/auth", "/users"]
        strip_path: true
  - name: bff-service
    url: http://bff-service:8080
    routes:
      - name: commerce-api
        paths: ["/api"]
        strip_path: true
```

`compose.foundation.yml` must define exactly these infrastructure services and one shared network: `postgres`, `redis`, `rabbitmq`, `kong`. Mount the SQL init directory into PostgreSQL and `kong.yaml` read-only into Kong. Publish only PostgreSQL, Redis and RabbitMQ ports needed by a locally-run service, Kong ports `8000` and `8001`, and RabbitMQ management port `15672`.

- [ ] **Step 4: Validate the Compose graph**

Run: `docker compose -f deployments/docker/compose.foundation.yml config --quiet`

Expected: exit code 0.

- [ ] **Step 5: Start infrastructure and check health**

Run: `docker compose -f deployments/docker/compose.foundation.yml up -d`

Run: `docker compose -f deployments/docker/compose.foundation.yml ps`

Expected: PostgreSQL, Redis, RabbitMQ and Kong are running; no application service is required yet.

- [ ] **Step 6: Commit infrastructure**

```powershell
git add deployments scripts/verify-foundation.ps1
git commit -m "chore: add local commerce infrastructure"
```

### Task 3: Create canonical protobuf contracts and deterministic code generation

**Files:**

- Create: `buf.yaml`
- Create: `buf.gen.yaml`
- Create: `contracts/proto/common/v1/common.proto`
- Create: `contracts/proto/catalog/v1/catalog.proto`
- Create: `contracts/proto/inventory/v1/inventory.proto`
- Create: `contracts/proto/order/v1/order.proto`
- Create: `contracts/proto/identity/v1/identity.proto`
- Create: `contracts/go/go.mod`
- Create: `contracts/java/pom.xml`

- [ ] **Step 1: Write an invalid proto file and prove lint catches it**

Create `contracts/proto/common/v1/common.proto` temporarily without a package declaration, then run:

```powershell
buf lint
```

Expected: failure for package/layout linting.

- [ ] **Step 2: Define the Buf v2 module and code generation configuration**

`buf.yaml`:

```yaml
version: v2
modules:
  - path: contracts/proto
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

`buf.gen.yaml` uses Buf v2 remote plugins, pinned to explicit versions selected from the public Buf registry at implementation time:

```yaml
version: v2
clean: true
plugins:
  - remote: buf.build/protocolbuffers/go
    out: contracts/go
    opt: paths=source_relative
  - remote: buf.build/grpc/go
    out: contracts/go
    opt: paths=source_relative
  - remote: buf.build/protocolbuffers/java
    out: contracts/java/src/main/generated
  - remote: buf.build/grpc/java
    out: contracts/java/src/main/generated
```

Use package and language options directly in each `.proto`. Example `common.proto`:

```proto
syntax = "proto3";

package ecommerce.common.v1;

option go_package = "ecommerce.local/contracts/go/common/v1;commonv1";
option java_multiple_files = true;
option java_package = "com.ecommerce.contracts.common.v1";

message Money {
  string currency = 1;
  int64 amount_minor = 2;
}
```

Define only one read/command RPC per domain in this phase: `GetProduct`, `GetAvailability`, `CreateOrder`, and `GetUser`. Reserve deleted field numbers rather than reusing them.

- [ ] **Step 3: Add contract build modules**

`contracts/go/go.mod`:

```go
module ecommerce.local/contracts/go

go 1.24
```

`contracts/java/pom.xml` is a Java 21 JAR module named `com.ecommerce:contracts`. It includes `protobuf-java`, `grpc-protobuf` and `grpc-stub` dependencies at one pinned gRPC version shared with Java services, and treats `src/main/generated` as a generated-source root.

- [ ] **Step 4: Verify lint and generation**

Run: `buf lint`

Run: `buf generate`

Expected: lint passes and Go/Java generated source appears under `contracts/go/` and `contracts/java/src/main/generated/` with no manual edit.

- [ ] **Step 5: Commit contracts and generated-output policy**

```powershell
git add buf.yaml buf.gen.yaml buf.lock contracts/proto contracts/go/go.mod contracts/java/pom.xml
git commit -m "feat: add v1 service contracts"
```

### Task 4: Add Go service skeletons with health and configuration tests

**Files:**

- Create: `services/bff-service/{go.mod,cmd/server/main.go,internal/config/config.go,internal/http/health.go}`
- Create: `services/catalog-service/{go.mod,cmd/server/main.go,internal/config/config.go,internal/http/health.go}`
- Create: `services/inventory-service/{go.mod,cmd/server/main.go,internal/config/config.go,internal/http/health.go}`
- Create: `services/*-service/internal/config/config_test.go`

- [ ] **Step 1: Write a failing config test in each Go service**

Use the same behavior in each `config_test.go`: constructing configuration with no `HTTP_ADDR` must return an error; `HTTP_ADDR=:8080` must return a config with port `:8080`.

```go
func TestLoadRequiresHTTPAddr(t *testing.T) {
    t.Setenv("HTTP_ADDR", "")
    _, err := Load()
    if err == nil { t.Fatal("expected HTTP_ADDR validation error") }
}
```

- [ ] **Step 2: Run the test and confirm the package is missing**

Run: `go test ./services/bff-service/...`

Expected: failure because the module and `Load` function do not exist.

- [ ] **Step 3: Implement identical bounded skeleton behavior with service-specific names**

Each Go module has a local module name (`ecommerce.local/bff-service`, `ecommerce.local/catalog-service`, `ecommerce.local/inventory-service`). `config.Load()` reads `HTTP_ADDR` and rejects blank values. `main.go` starts an `http.Server`, registers only `GET /healthz`, and returns:

```json
{"service":"catalog-service","status":"ok"}
```

Use `:8080`, `:8081`, and `:8082` as default listen addresses. Docker callers use `bff-service:8080`, `catalog-service:8081`, and `inventory-service:8082`; a server must not bind to its Docker DNS name. Do not connect to Redis, PostgreSQL, RabbitMQ or gRPC in this task.

- [ ] **Step 4: Run unit tests and launch each health endpoint**

Run: `go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...`

Expected: PASS.

Run each `go run ./cmd/server` with its `HTTP_ADDR`, then call `GET /healthz`.

Expected: HTTP 200 and the correct service name.

- [ ] **Step 5: Commit Go skeletons**

```powershell
git add go.work services/bff-service services/catalog-service services/inventory-service
git commit -m "feat: add Go service health skeletons"
```

### Task 5: Add Java Identity and Order skeletons with Actuator health

**Files:**

- Create: `pom.xml`
- Create: `services/identity-service/pom.xml`
- Create: `services/order-service/pom.xml`
- Create: `services/identity-service/src/main/java/com/ecommerce/identity/IdentityApplication.java`
- Create: `services/order-service/src/main/java/com/ecommerce/order/OrderApplication.java`
- Create: `services/{identity,order}-service/src/main/resources/application.yml`
- Create: `services/{identity,order}-service/src/test/java/.../*ApplicationTest.java`

- [ ] **Step 1: Write failing Spring context tests**

Each test must use `@SpringBootTest` and assert the application context starts. Identity test also asserts Actuator exposes `/actuator/health`.

```java
@SpringBootTest
class IdentityApplicationTest {
  @Test void contextLoads() {}
}
```

- [ ] **Step 2: Run the test before the applications exist**

Run: `mvn -pl services/identity-service test`

Expected: Maven fails because the reactor/module does not exist.

- [ ] **Step 3: Create the Java 21 Maven reactor and applications**

Root `pom.xml` has packaging `pom`, modules `contracts/java`, `services/identity-service`, `services/order-service`, and shared Java 21/Spring Boot/gRPC properties. Each service uses Spring Boot Web, Actuator, Validation and its own package root. Configure health exposure only:

```yaml
server:
  port: 8080
management:
  endpoints:
    web:
      exposure:
        include: health,info
  endpoint:
    health:
      probes:
        enabled: true
```

Identity's package must not yet issue tokens; create only application startup and health. Order uses port `8083`. Database connections, password hashing and JWT code belong to Phase 1.

- [ ] **Step 4: Run tests and health smoke checks**

Run: `mvn test`

Expected: PASS for contract and both Spring modules.

Run: `mvn -pl services/identity-service spring-boot:run`

Expected: `GET http://localhost:8080/actuator/health` returns `{"status":"UP"}`.

- [ ] **Step 5: Commit Java skeletons**

```powershell
git add pom.xml contracts/java services/identity-service services/order-service
git commit -m "feat: add Java service health skeletons"
```

### Task 6: Wire service containers and Kong routes without exposing internal services

**Files:**

- Modify: `deployments/docker/compose.foundation.yml`
- Modify: `deployments/kong/kong.yaml`
- Create: `services/{bff,catalog,inventory}-service/Dockerfile`
- Create: `services/{identity,order}-service/Dockerfile`
- Modify: `scripts/verify-foundation.ps1`

- [ ] **Step 1: Add a failing route smoke test**

Append a check to `scripts/verify-foundation.ps1` that, after Compose is up, requires `http://localhost:8000/api/healthz` and `http://localhost:8000/auth/actuator/health` to return non-5xx responses.

- [ ] **Step 2: Run the route check before application containers are added**

Run: `make up`

Run: `pwsh -File scripts/verify-foundation.ps1`

Expected: route smoke check fails because Kong upstreams do not exist.

- [ ] **Step 3: Containerize all five services and add only public routes**

Each Dockerfile must use a multi-stage build and run as a non-root user where the chosen base image permits it. Add all five service containers to the same Compose network. Do not publish host ports for any application service: Kong is the only client ingress. Catalog, Order and Inventory remain network-only; use `docker compose exec` for debug.

Update `kong.yaml` so:

- `/auth` and `/users` route to Identity;
- `/api` routes to BFF;
- Catalog, Order and Inventory have no Kong route;
- Kong's JWT enforcement is not enabled until Identity can issue valid tokens in Phase 1.

- [ ] **Step 4: Run container and route smoke checks**

Run: `make up`

Run: `docker compose -f deployments/docker/compose.foundation.yml ps`

Run: `pwsh -File scripts/verify-foundation.ps1`

Expected: all containers are healthy; Buf lint passes; health checks through Kong pass; no internal service has a public route.

- [ ] **Step 5: Commit deployment wiring**

```powershell
git add deployments services scripts/verify-foundation.ps1
git commit -m "feat: run service skeletons through Kong"
```

### Task 7: Write the runnable hand-off and enforce the complete foundation check

**Files:**

- Create: `README.md`
- Modify: `scripts/verify-foundation.ps1`
- Modify: `docs/superpowers/specs/2026-07-30-ecommerce-microservices-roadmap-design.md`

- [ ] **Step 1: Write a failing documentation assertion**

Add to `scripts/verify-foundation.ps1` a required-text check for `make up`, `make proto`, `make verify`, the five service names and the two public Kong prefixes in `README.md`.

- [ ] **Step 2: Run it and confirm README is missing**

Run: `pwsh -File scripts/verify-foundation.ps1`

Expected: failure stating that `README.md` or required commands are absent.

- [ ] **Step 3: Write the operational README**

README must contain these exact sections: `Prerequisites`, `Quick start`, `Service map`, `Public routes`, `Local development`, `Verification`, `Security boundary`, and `Known phase-0 limitations`.

It must document:

- copy `.env.example` to `.env` and replace local-only passwords;
- `make proto`, `make up`, `make verify`, `make down`;
- Kong exposes only `/auth`, `/users` and `/api`;
- Identity has no functional login/JWT yet in Phase 0;
- no command uses `docker compose down -v` by default.

- [ ] **Step 4: Execute the full verification sequence**

Run: `make proto`

Run: `make up`

Run: `make verify`

Run: `mvn test`

Run: `go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...`

Expected: every command exits 0.

- [ ] **Step 5: Commit the phase hand-off**

```powershell
git add README.md scripts/verify-foundation.ps1 docs/superpowers/specs
git commit -m "docs: complete phase 0 foundation handoff"
```

## Final acceptance checklist

- [ ] `buf lint` and `buf generate` run from repository root.
- [ ] Generated Go and Java contracts compile in their respective build systems.
- [ ] PostgreSQL has isolated logical databases for Identity, Order, Catalog and Inventory.
- [ ] Kong only routes BFF and Identity; internal services cannot be called from a host port or a Kong route.
- [ ] Every one of the five service containers has a health endpoint and a smoke check.
- [ ] `make verify`, Maven tests and Go tests pass from a clean clone.
- [ ] No private signing key, password, token or `.env` file is tracked by Git.
