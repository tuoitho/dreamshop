# Monorepo Steering Documentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish progressive-disclosure steering instructions for the repository, its five backend services, and the React frontend without weakening microservice boundaries or duplicating canonical rules.

**Architecture:** The root `AGENTS.md` is the mandatory entry point, `docs/steering/README.md` routes agents to focused canonical documents, and local `AGENTS.md` files specialize those rules for the nearest application surface. Repository-wide steering is completed and validated before service/frontend instructions are added.

**Tech Stack:** Markdown, PowerShell validation, Java 21/Maven, Go 1.24, React 19/TypeScript/Vite/Vitest, Protobuf/Buf, Docker Compose, Kong.

## Global Constraints

- Treat every backend service as an independently deployable bounded context.
- Never introduce direct source imports, database access, foreign keys, runtime joins, or database transactions across service boundaries.
- Use direct private gRPC or RabbitMQ events for internal communication; do not route service-to-service traffic through Kong or BFF.
- Keep current public contracts backward compatible unless a task explicitly authorizes a breaking change.
- Preserve the REST success envelope `{ "data": <payload>, "message": "..." }` and error envelope `{ "code": "STABLE_ERROR_CODE", "message": "...", "errors": {}, "traceId": "..." }`.
- Keep REST/JSON, Java, and TypeScript fields in `camelCase`; protobuf fields and database columns use `lower_snake_case`; exported Go identifiers use `PascalCase`.
- ADR-001, ADR-006, and ADR-008 remain authoritative architectural decisions.
- Local instructions may specialize but must not relax repository-wide boundaries, compatibility, security, or verification rules.
- Describe current services and accepted ownership only; do not invent payment, notification, cart, review, or search services.
- Do not modify runtime code, API schemas, generated contracts, deployment configuration, or unrelated documentation as part of this plan.

---

## File Map

- Modify `AGENTS.md`: mandatory repository-wide entry point and routing links.
- Create `docs/steering/README.md`: progressive-disclosure routing index.
- Create `docs/steering/architecture.md`: ownership, autonomy, database, and shared-code rules.
- Create `docs/steering/service-boundaries.md`: cross-service dependency, workflow, data, and transaction rules.
- Create `docs/steering/api-contracts.md`: REST, protobuf, gRPC, event, compatibility, error, and idempotency rules.
- Create `docs/steering/change-impact.md`: `DIRECT`, `CONTRACT`, `DEPENDENT`, `INFRA`, and `TEST` impact classification.
- Create `docs/steering/testing.md`: unit, integration, contract, E2E, regression, and progressive verification guidance.
- Create `docs/steering/delivery.md`: migration, config, secrets, deployment, observability, and rollout guidance.
- Create `services/bff-service/AGENTS.md`: BFF ownership, composition boundaries, Go conventions, and checks.
- Create `services/catalog-service/AGENTS.md`: catalog ownership, cache/data boundaries, Go conventions, and checks.
- Create `services/identity-service/AGENTS.md`: identity/security ownership, Java architecture, persistence, and checks.
- Create `services/inventory-service/AGENTS.md`: stock/reservation ownership, consumer rules, Go conventions, and checks.
- Create `services/order-service/AGENTS.md`: order lifecycle/idempotency/outbox ownership, Java conventions, and checks.
- Create `ecommerce-fe/AGENTS.md`: frontend ownership, session/API/UI boundaries, TypeScript conventions, and checks.

## Task 1: Root Entry Point and Disclosure Index

**Files:**
- Modify: `AGENTS.md`
- Create: `docs/steering/README.md`

**Interfaces:**
- Consumes: repository structure and commands from `README.md`, `Makefile`, `pom.xml`, `go.work`, and `ecommerce-fe/package.json`; architecture decisions from ADR-001, ADR-006, and ADR-008.
- Produces: the mandatory rule set and canonical navigation paths consumed by every later steering document and local `AGENTS.md`.

- [ ] **Step 1: Refactor the root instruction outline**

  Replace the current long-form structure with these focused sections: `Repository Instructions`, `Core Rules`, `Repository Map`, `Before Modifying Code`, `Instruction Routing`, `Commands`, `Naming`, and `REST Response Contract`. Keep the existing verified commands and response envelopes, add the explicit monorepo-boundary statement, and link all seven files under `docs/steering/`.

- [ ] **Step 2: Add the routing index**

  Create `docs/steering/README.md` with a table containing these exact routing categories: ownership/service interaction, REST/gRPC/protobuf/event contract, cross-surface impact, tests/bug fixes, and migration/config/deployment. State that agents always read root instructions, then the nearest local `AGENTS.md`, then only the topic documents selected by the table.

- [ ] **Step 3: Validate paths and mandatory root rules**

  Run:

  ```powershell
  $requiredLinks = @(
    'docs/steering/README.md',
    'docs/steering/architecture.md',
    'docs/steering/service-boundaries.md',
    'docs/steering/api-contracts.md',
    'docs/steering/change-impact.md',
    'docs/steering/testing.md',
    'docs/steering/delivery.md'
  )
  $rootText = Get-Content -Raw AGENTS.md
  $missingLinks = $requiredLinks | Where-Object { $rootText -notmatch [regex]::Escape($_) }
  if ($missingLinks) { throw "AGENTS.md is missing steering links: $($missingLinks -join ', ')" }
  Select-String -Path AGENTS.md -Pattern 'Monorepo.*not.*monolith|direct imports|database|gRPC|RabbitMQ|traceId'
  git diff --check
  ```

  Expected: no missing-link exception, mandatory rules are matched, and `git diff --check` exits successfully. Topic targets are created by Tasks 2 and 3 before Phase 1 validation.

- [ ] **Step 4: Commit the entry point and index**

  ```powershell
  git add -- AGENTS.md docs/steering/README.md
  git commit -m "docs: add steering entry point"
  ```

## Task 2: Architecture, Boundaries, and Impact Analysis

**Files:**
- Create: `docs/steering/architecture.md`
- Create: `docs/steering/service-boundaries.md`
- Create: `docs/steering/change-impact.md`

**Interfaces:**
- Consumes: root core rules and ADR-001/ADR-008 ownership and routing decisions.
- Produces: canonical ownership/boundary rules and impact categories referenced by contract, delivery, and local instructions.

- [ ] **Step 1: Write architecture ownership rules**

  Define one owner for each accepted capability: BFF for client-specific commerce composition and REST adaptation; Identity for credentials, profile, roles, sessions, refresh tokens, and JWT issuance; Catalog for product/catalog data and cache; Order for order lifecycle, idempotency, and outbox; Inventory for stock, reservations, and inventory event consumption. Add database ownership, independent deployability/testability/scalability/observability, and allowed versus forbidden shared-code examples.

- [ ] **Step 2: Write service-boundary rules**

  State the prohibited dependencies explicitly: service implementation imports, another service's database/schema, cross-service foreign keys, runtime database joins, and distributed database transactions. Define direct private gRPC for immediate responses, RabbitMQ events when immediate consistency is unnecessary, small read-model duplication, duplicate-safe consumers, retries, compensation, and saga/workflow coordination. Preserve the rule that internal traffic bypasses Kong and BFF.

- [ ] **Step 3: Write change-impact classification**

  Define `DIRECT`, `CONTRACT`, `DEPENDENT`, `INFRA`, and `TEST` with repository-specific examples. Require the inventory before implementation, require inspection but not automatic modification of dependents, and require single-service changes to stay inside that service when possible.

- [ ] **Step 4: Check ownership and boundary consistency**

  Run:

  ```powershell
  rg -n "BFF|Identity|Catalog|Order|Inventory|database|gRPC|RabbitMQ|DIRECT|CONTRACT|DEPENDENT|INFRA|TEST" docs/steering/architecture.md docs/steering/service-boundaries.md docs/steering/change-impact.md
  rg -n "payment-service|notification-service|auth-service|packages/contracts|infra/" docs/steering/architecture.md docs/steering/service-boundaries.md docs/steering/change-impact.md
  git diff --check
  ```

  Expected: all current owners, communication paths, and impact categories are present; the second search returns no matches; diff check passes.

- [ ] **Step 5: Commit the architecture steering set**

  ```powershell
  git add -- docs/steering/architecture.md docs/steering/service-boundaries.md docs/steering/change-impact.md
  git commit -m "docs: define service boundaries and impact analysis"
  ```

## Task 3: Contract, Testing, and Delivery Steering

**Files:**
- Create: `docs/steering/api-contracts.md`
- Create: `docs/steering/testing.md`
- Create: `docs/steering/delivery.md`

**Interfaces:**
- Consumes: root response/naming rules, Task 2 boundary rules, `buf.yaml`, `buf.gen.yaml`, `contracts/`, repository build files, and ADR-006/ADR-008.
- Produces: canonical implementation and verification procedures referenced by all local instructions.

- [ ] **Step 1: Write contract rules against the actual repository layout**

  Identify `contracts/proto/` as protobuf source, `contracts/java/` and `contracts/go/` as generated modules, and `contracts/openapi.yaml` as the conventional HTTP contract location only when its content represents the e-commerce API being changed. Require `lower_snake_case` protobuf fields, additive compatibility by default, explicit migration/versioning for breaking changes, `buf lint`, `buf generate`, consumer inspection, tolerant event consumers, stable event metadata, and duplicate-safe handling. Do not claim the current unrelated scaffold OpenAPI content is an authoritative e-commerce contract.

- [ ] **Step 2: Preserve public response, error, and idempotency rules**

  Document the success/pagination/error envelopes exactly as stated in Global Constraints, meaningful mutation success messages, stable business error codes, camelCase `traceId` at the REST boundary, gRPC status plus versioned reason internally, no exposed stack traces, and request fingerprint semantics for idempotency keys.

- [ ] **Step 3: Write testing guidance**

  Define smallest-sufficient unit, focused Spring slice, database/message/API integration, independent contract, and critical-flow E2E scopes. Preserve the exact Maven method → class → affected reactor sequence from root instructions, test log capture guidance, Go `go test` commands, frontend Vitest/Testing Library commands, and regression-first bug fixing.

- [ ] **Step 4: Write delivery guidance**

  Cover service-owned migrations, expand/migrate/contract sequencing for incompatible data changes, backward-compatible mixed-version rollout, environment variable and secret handling, deployment/Kong impact, observability and correlation, Docker Compose infrastructure with host-run applications, `host.docker.internal:<port>`, and evidence-based handoff when an environmental dependency blocks broad verification.

- [ ] **Step 5: Validate canonical terminology**

  Run:

  ```powershell
  rg -n "contracts/proto|contracts/java|contracts/go|contracts/openapi.yaml|buf lint|buf generate" docs/steering/api-contracts.md
  rg -n '"data"|"message"|"code"|traceId|Idempotency-Key' docs/steering/api-contracts.md
  rg -n "mvn test|go test|npm --prefix ecommerce-fe|Testcontainers|contract|E2E" docs/steering/testing.md
  rg -n "migration|secret|Docker Compose|host.docker.internal|observability" docs/steering/delivery.md
  git diff --check
  ```

  Expected: every canonical term is present and diff check passes.

- [ ] **Step 6: Commit the implementation steering set**

  ```powershell
  git add -- docs/steering/api-contracts.md docs/steering/testing.md docs/steering/delivery.md
  git commit -m "docs: add contract testing and delivery steering"
  ```

## Task 4: Phase 1 Consistency Gate

**Files:**
- Verify: `AGENTS.md`
- Verify: `docs/steering/*.md`

**Interfaces:**
- Consumes: Tasks 1–3.
- Produces: a stable repository-wide instruction layer required before local instructions are created.

- [ ] **Step 1: Verify every canonical file and root link exists**

  ```powershell
  $requiredFiles = @(
    'docs/steering/README.md',
    'docs/steering/architecture.md',
    'docs/steering/service-boundaries.md',
    'docs/steering/api-contracts.md',
    'docs/steering/change-impact.md',
    'docs/steering/testing.md',
    'docs/steering/delivery.md'
  )
  $missingFiles = $requiredFiles | Where-Object { -not (Test-Path -LiteralPath $_) }
  if ($missingFiles) { throw "Missing steering files: $($missingFiles -join ', ')" }
  $rootText = Get-Content -Raw AGENTS.md
  $unlinkedFiles = $requiredFiles | Where-Object { $rootText -notmatch [regex]::Escape($_) }
  if ($unlinkedFiles) { throw "Unlinked steering files: $($unlinkedFiles -join ', ')" }
  ```

  Expected: no exception.

- [ ] **Step 2: Scan for rule drift and conversation residue**

  ```powershell
  rg -n "payment-service|notification-service|auth-service|packages/contracts|infra/" AGENTS.md docs/steering
  $residuePatterns = @(
    ('as' + ' requested'),
    ('per your' + ' instruction'),
    ('now self' + '-contained'),
    ('consolidated' + ' from'),
    ('T' + 'BD'),
    ('TO' + 'DO'),
    ('PLACE' + 'HOLDER')
  )
  Select-String -Path AGENTS.md,docs/steering/*.md -Pattern $residuePatterns
  rg -n "trace_id|full_name|page_size|total_items|total_pages" AGENTS.md docs/steering
  git diff --check
  ```

  Expected: no stale example-service paths, prompt residue, unfinished markers, or snake_case REST fields. References to `trace_id` are allowed only when explicitly describing internal gRPC/log metadata rather than REST JSON.

- [ ] **Step 3: Review Phase 1 as a standalone instruction system**

  Confirm manually that root instructions answer what always applies, the index answers what to read next, each detailed rule has one canonical home, and ADR links support rather than duplicate architectural rationale. Amend the relevant Task 1–3 commit if a correction is needed; do not create a correction-only commit.

## Task 5: BFF Local Instructions

**Files:**
- Create: `services/bff-service/AGENTS.md`

**Interfaces:**
- Consumes: root rules plus architecture, service-boundary, API-contract, testing, and delivery steering.
- Produces: scoped rules for files under `services/bff-service/`.

- [ ] **Step 1: Document BFF ownership and exclusions**

  State that BFF owns client-specific commerce composition, REST adaptation, custom error mapping, and orchestration when a route is not a simple approved transcoding. State that it does not own catalog, inventory, order, or identity domain rules; it does not access service databases; and it calls domain services directly over private gRPC rather than through Kong.

- [ ] **Step 2: Document current Go layout and verification**

  Reference `cmd/server/`, `internal/config/`, and `internal/http/`; require configuration through environment-backed config; keep exported/unexported naming idiomatic; place `_test.go` beside packages. Include `go test ./...` from the service directory and `go test ./services/bff-service/...` from repository root, plus relevant links using `../../docs/steering/...`.

- [ ] **Step 3: Validate and commit**

  ```powershell
  rg -n "composition|REST|gRPC|Kong|database|go test|../../docs/steering" services/bff-service/AGENTS.md
  git diff --check
  git add -- services/bff-service/AGENTS.md
  git commit -m "docs: add BFF service instructions"
  ```

## Task 6: Catalog Local Instructions

**Files:**
- Create: `services/catalog-service/AGENTS.md`

**Interfaces:**
- Consumes: root rules plus architecture, service-boundary, API-contract, testing, and delivery steering.
- Produces: scoped rules for files under `services/catalog-service/`.

- [ ] **Step 1: Document Catalog ownership and exclusions**

  State that Catalog owns product/catalog data and catalog caching. It does not own stock availability, reservations, order lifecycle, user identity, or client-specific composition. Other services consume catalog contracts or APIs rather than its source or persistence.

- [ ] **Step 2: Document current Go layout and verification**

  Reference `cmd/server/`, `internal/config/`, and `internal/http/`; keep infrastructure adapters outside domain rules as the service grows; require `_test.go` files beside packages. Include `go test ./...` from the service directory and `go test ./services/catalog-service/...` from root, plus relevant `../../docs/steering/...` links.

- [ ] **Step 3: Validate and commit**

  ```powershell
  rg -n "product|catalog|cache|inventory|order|identity|go test|../../docs/steering" services/catalog-service/AGENTS.md
  git diff --check
  git add -- services/catalog-service/AGENTS.md
  git commit -m "docs: add catalog service instructions"
  ```

## Task 7: Identity Local Instructions

**Files:**
- Create: `services/identity-service/AGENTS.md`

**Interfaces:**
- Consumes: root rules plus architecture, service-boundary, API-contract, testing, and delivery steering; current Spring packages and Flyway migrations.
- Produces: scoped rules for files under `services/identity-service/`.

- [ ] **Step 1: Document Identity ownership and security boundaries**

  State that Identity owns credentials, registration/login, profile, role, refresh-token sessions, JWT issuance, and JWKS. It does not own commerce authorization decisions, catalog, inventory, or order state. Require private signing keys to stay out of source control, access tokens to remain short-lived, refresh rotation/reuse behavior to preserve session security, and callers to consume identity APIs/JWT claims rather than identity persistence.

- [ ] **Step 2: Document Java architecture, persistence, and tests**

  Reference the current `auth`, `user`, `location`, `security`, and `common` packages without imposing a new package reorganization. Keep domain/application logic separate from controllers and JPA entities, use Flyway for schema changes, prefer plain JUnit/Mockito and focused Spring slices, and reserve Testcontainers for real persistence. Include method, class, and `mvn -pl services/identity-service -am test` verification levels plus relevant `../../docs/steering/...` links.

- [ ] **Step 3: Validate and commit**

  ```powershell
  rg -n "credential|profile|role|refresh|JWT|JWKS|Flyway|Testcontainers|mvn -pl services/identity-service|../../docs/steering" services/identity-service/AGENTS.md
  git diff --check
  git add -- services/identity-service/AGENTS.md
  git commit -m "docs: add identity service instructions"
  ```

## Task 8: Inventory Local Instructions

**Files:**
- Create: `services/inventory-service/AGENTS.md`

**Interfaces:**
- Consumes: root rules plus architecture, service-boundary, API-contract, testing, and delivery steering.
- Produces: scoped rules for files under `services/inventory-service/`.

- [ ] **Step 1: Document Inventory ownership and workflow boundaries**

  State that Inventory owns stock, reservation lifecycle, availability invariants, and inventory event consumption. It does not own product descriptions/pricing, order lifecycle, identity, or client orchestration. Require atomic inventory invariants inside its persistence boundary and duplicate-safe event processing; order state changes happen through contracts/events rather than direct access.

- [ ] **Step 2: Document current Go layout and verification**

  Reference `cmd/server/`, `internal/config/`, and `internal/http/`; require broker/database adapters to remain behind service-owned interfaces when introduced; keep `_test.go` files beside packages. Include `go test ./...` from the service directory and `go test ./services/inventory-service/...` from root, plus relevant `../../docs/steering/...` links.

- [ ] **Step 3: Validate and commit**

  ```powershell
  rg -n "stock|reservation|idempot|event|order|catalog|go test|../../docs/steering" services/inventory-service/AGENTS.md
  git diff --check
  git add -- services/inventory-service/AGENTS.md
  git commit -m "docs: add inventory service instructions"
  ```

## Task 9: Order Local Instructions

**Files:**
- Create: `services/order-service/AGENTS.md`

**Interfaces:**
- Consumes: root rules plus architecture, service-boundary, API-contract, testing, and delivery steering; current Spring Boot skeleton and accepted ADR ownership.
- Produces: scoped rules for files under `services/order-service/`.

- [ ] **Step 1: Document Order ownership and workflow boundaries**

  State that Order owns order lifecycle, order state transitions, command idempotency, and transactional outbox state. It does not own stock/reservation invariants, catalog product truth, identity credentials, or future payment behavior. Require inventory interaction through gRPC/events, local transactions only, and compensation/saga semantics for multi-service workflows.

- [ ] **Step 2: Document Java conventions and verification**

  Require domain/application logic to remain independent of controllers, messaging adapters, and persistence as those layers are introduced; use Java 21, Lombok for boilerplate, JUnit 5, focused Spring tests, and Testcontainers only when real infrastructure is needed. Include method/class progression and `mvn -pl services/order-service -am test` from root, plus relevant `../../docs/steering/...` links.

- [ ] **Step 3: Validate and commit**

  ```powershell
  rg -n "order lifecycle|idempot|outbox|inventory|transaction|saga|mvn -pl services/order-service|../../docs/steering" services/order-service/AGENTS.md
  git diff --check
  git add -- services/order-service/AGENTS.md
  git commit -m "docs: add order service instructions"
  ```

## Task 10: Frontend Local Instructions

**Files:**
- Create: `ecommerce-fe/AGENTS.md`

**Interfaces:**
- Consumes: root response/naming/security rules, API-contract and testing steering, `ecommerce-fe/package.json`, and current `src/` layout.
- Produces: scoped rules for files under `ecommerce-fe/`.

- [ ] **Step 1: Document frontend ownership and backend boundaries**

  State that the frontend owns presentation, client routing, accessible interaction, view state, API adaptation, and session UX. It does not own identity, catalog, inventory, or order business truth; client validation supplements but does not replace server validation. Require public API calls through Kong, contract-aligned camelCase types, stable error-code handling, and mutation success messages for notifications.

- [ ] **Step 2: Preserve session and source-layout conventions**

  Document the current `components/`, `context/`, `hooks/`, `lib/`, `pages/`, and `routes/` responsibilities. Preserve in-memory access tokens, HttpOnly refresh cookies, one refresh/retry attempt, and shared concurrent refresh behavior. Require reuse of `components/ui/`, user-facing behavior tests with Testing Library, and `*.test.ts`/`*.test.tsx` placement under `src/`.

- [ ] **Step 3: Add exact frontend verification commands**

  Include these commands from repository root:

  ```powershell
  npm --prefix ecommerce-fe run test
  npm --prefix ecommerce-fe run lint
  npm --prefix ecommerce-fe run build
  ```

  Link to `../docs/steering/api-contracts.md`, `../docs/steering/testing.md`, `../docs/steering/change-impact.md`, and `../docs/steering/delivery.md`.

- [ ] **Step 4: Validate and commit**

  ```powershell
  rg -n "presentation|Kong|camelCase|HttpOnly|refresh|Testing Library|npm --prefix ecommerce-fe|../docs/steering" ecommerce-fe/AGENTS.md
  git diff --check
  git add -- ecommerce-fe/AGENTS.md
  git commit -m "docs: add frontend instructions"
  ```

## Task 11: Final Repository-wide Verification

**Files:**
- Verify: `AGENTS.md`
- Verify: `docs/steering/*.md`
- Verify: `services/*/AGENTS.md`
- Verify: `ecommerce-fe/AGENTS.md`

**Interfaces:**
- Consumes: all earlier tasks.
- Produces: evidence that the progressive-disclosure hierarchy is complete, internally consistent, and backed by valid repository commands.

- [ ] **Step 1: Verify the complete hierarchy**

  ```powershell
  $requiredFiles = @(
    'AGENTS.md',
    'docs/steering/README.md',
    'docs/steering/architecture.md',
    'docs/steering/service-boundaries.md',
    'docs/steering/api-contracts.md',
    'docs/steering/change-impact.md',
    'docs/steering/testing.md',
    'docs/steering/delivery.md',
    'services/bff-service/AGENTS.md',
    'services/catalog-service/AGENTS.md',
    'services/identity-service/AGENTS.md',
    'services/inventory-service/AGENTS.md',
    'services/order-service/AGENTS.md',
    'ecommerce-fe/AGENTS.md'
  )
  $missingFiles = $requiredFiles | Where-Object { -not (Test-Path -LiteralPath $_) }
  if ($missingFiles) { throw "Missing instruction files: $($missingFiles -join ', ')" }
  ```

  Expected: no exception.

- [ ] **Step 2: Verify local links and prohibited residue**

  ```powershell
  $localFiles = Get-ChildItem services,ecommerce-fe -Filter AGENTS.md -Recurse
  foreach ($file in $localFiles) {
    $text = Get-Content -Raw $file.FullName
    $links = [regex]::Matches($text, '\]\(([^)#]+\.md)\)') | ForEach-Object { $_.Groups[1].Value }
    foreach ($link in $links) {
      $target = Join-Path $file.DirectoryName $link
      if (-not (Test-Path -LiteralPath $target)) { throw "Broken link in $($file.FullName): $link" }
    }
  }
  $residuePatterns = @(
    ('as' + ' requested'),
    ('per your' + ' instruction'),
    ('now self' + '-contained'),
    ('consolidated' + ' from'),
    ('T' + 'BD'),
    ('TO' + 'DO'),
    ('PLACE' + 'HOLDER')
  )
  $instructionFiles = @('AGENTS.md', 'ecommerce-fe/AGENTS.md') +
    (Get-ChildItem docs/steering,services -Filter '*.md' -Recurse | Select-Object -ExpandProperty FullName)
  Select-String -Path $instructionFiles -Pattern $residuePatterns
  git diff --check
  ```

  Expected: all local Markdown links resolve, residue search returns no matches, and diff check passes.

- [ ] **Step 3: Run the documented static and test commands**

  ```powershell
  buf lint
  go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
  mvn -pl services/identity-service,services/order-service -am test
  npm --prefix ecommerce-fe run test
  npm --prefix ecommerce-fe run lint
  npm --prefix ecommerce-fe run build
  ```

  Expected: all commands pass. If Docker/Testcontainers or another external dependency blocks Maven integration tests, capture the exact failing dependency and report which narrower checks passed; do not claim the full Maven verification succeeded.

- [ ] **Step 4: Inspect scope and history**

  ```powershell
  git status --short
  git log --oneline -12
  git diff HEAD~10..HEAD --stat
  ```

  Expected: only the planned instruction files and plan/spec history are present; each task has a focused commit; no runtime or contract artifact changed.
