# Developer Harness and CI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the broken verification wrapper and add an accurate service catalog, progressively disclosed developer instructions, contribution rules, and repository-native GitHub Actions checks for P0–P3.

**Architecture:** Keep mandatory agent rules, canonical steering, developer procedures, contribution policy, and executable automation in separate layers connected by concise indexes. CI invokes the workspace's existing Buf, Maven, Go, npm, and PowerShell entry points directly; it does not introduce a new universal harness.

**Tech Stack:** Markdown, PowerShell 7, GitHub Actions YAML, Buf, Java 21/Maven 3.9+, Go 1.24, Node.js 22.12+/npm, Docker Compose v2.

## Global Constraints

- Do not restore the deleted generic verification engine, add a harness manifest, or create a replacement wrapper.
- Do not delete untracked or historical material under `harness/reports/`.
- Do not change public API, protobuf, event, or REST contracts.
- Keep the source-of-truth order: root/local `AGENTS.md`, `docs/steering/`, `docs/instructions/`, `docs/rules/`, then executable build and CI files.
- Keep service boundaries intact; do not add cross-service imports, database access, or business-feature wiring.
- Use the status labels `implemented`, `configured`, and `accepted but not wired` when describing topology.
- Use Java 21 and Maven 3.9 or newer.
- Use Go 1.24 or newer within the 1.24 line used by the workspace.
- Use Node.js 22.12 or newer for Vite 8.
- Use Docker Desktop/Compose v2, Buf CLI, PowerShell 7, and GNU Make when Makefile shortcuts are used.
- The contribution workflow is short-lived feature branch, pull request into `main`, at least one approval, required CI, and squash merge only.
- Only the pull-request title is required to follow Conventional Commits; intermediate branch commits are exempt.
- Do not add Taskfile, pre-commit hooks, Husky, Lefthook, commitlint, AI prompts, slash commands, deployment automation, or image publishing.
- Preserve historical specs and plans even when they mention the former wrapper.

---

## File Map

- Delete `harness/verify.ps1`: remove the broken public verification entry point.
- Create `docs/steering/service-catalog.md`: canonical current topology and wiring status.
- Modify `docs/steering/README.md` and `AGENTS.md`: route readers to the catalog and developer instructions.
- Modify `.env.example`: make the host-run BFF bind to `8081`.
- Create `docs/instructions/README.md`: task-based developer-procedure index.
- Create `docs/instructions/onboarding.md`: prerequisites and first setup.
- Create `docs/instructions/local-development.md`: host-app/local-infrastructure workflow.
- Create `docs/instructions/build-and-test.md`: direct verification commands by scope.
- Create `docs/instructions/operations.md`: local diagnosis and safe shutdown runbook.
- Modify `README.md`: replace stale universal-harness guidance with concise canonical links and supported quick start.
- Create `docs/rules/README.md`: contribution-policy index.
- Create `docs/rules/commit-convention.md`: squash-title syntax and examples.
- Create `docs/rules/pull-requests.md`: branch, review, merge, and administrator settings.
- Create `docs/rules/ci.md`: required check names, responsibilities, and failure policy.
- Create `.github/pull_request_template.md`: impact and verification evidence form.
- Create `scripts/validate-pr-title.ps1`: local/CI pull-request-title validator.
- Create `.github/workflows/ci.yml`: direct contract, Java, Go, frontend, and title checks.

### Task 1: Remove the Broken Verification Wrapper

**Files:**
- Delete: `harness/verify.ps1`
- Inspect only: `.agents/skills/fullstack-feature/SKILL.md`
- Inspect only: `harness/reports/`

**Interfaces:**
- Consumes: the skill's existing fallback to repository-native commands when no canonical harness exists.
- Produces: absence of the tracked broken wrapper; no replacement executable.

- [ ] **Step 1: Capture the wrapper and report-directory state**

Run:

```powershell
git ls-files --error-unmatch harness/verify.ps1
git status --short -- harness/reports
Get-Content -Raw harness/verify.ps1
```

Expected: the wrapper is tracked and delegates to a missing skill script; report files, if present, are not selected for modification.

- [ ] **Step 2: Confirm the delegated engine is absent and fallback is documented**

Run:

```powershell
Test-Path .agents/skills/fullstack-feature/scripts/verify.ps1
rg -n "canonical harness|build files|fallback|verify.ps1" .agents/skills/fullstack-feature/SKILL.md
```

Expected: `Test-Path` prints `False`; the skill documents direct-command fallback behavior.

- [ ] **Step 3: Delete only the tracked wrapper**

Use `apply_patch` to delete `harness/verify.ps1`. Do not remove the `harness/` directory recursively and do not edit `harness/reports/`.

- [ ] **Step 4: Verify the scoped deletion and live-reference state**

Run:

```powershell
git status --short
git ls-files harness/verify.ps1
rg -n "harness/verify\.ps1" AGENTS.md README.md docs/steering .agents services ecommerce-fe scripts Makefile
```

Expected: Git shows only `D harness/verify.ps1`; `git ls-files` and the live-reference search return no matches. Matches in `docs/superpowers/specs/` or `docs/superpowers/plans/` are intentionally outside the search.

- [ ] **Step 5: Commit the removal**

```powershell
git add -- harness/verify.ps1
git commit -m "chore: remove broken verification wrapper"
```

Expected: one commit deletes the wrapper and does not include reports.

### Task 2: Add the Canonical Service Catalog

**Files:**
- Create: `docs/steering/service-catalog.md`
- Modify: `docs/steering/README.md`
- Modify: `AGENTS.md`
- Modify: `.env.example`
- Inspect: `deployments/docker/compose.foundation.yml`
- Inspect: `deployments/kong/kong.yml`
- Inspect: service Dockerfiles, launchers, and runtime configuration files

**Interfaces:**
- Consumes: current Compose, Kong, launcher, Dockerfile, and application configuration.
- Produces: one canonical topology table and a local BFF address of `:8081` used by later developer instructions.

- [ ] **Step 1: Reconfirm topology from executable configuration**

Run:

```powershell
rg -n "8080|8081|8082|8083|50051|5173|5432|6379|5672|15672|healthz|actuator/health|identity_db|order_db|catalog_db|inventory_db" deployments services ecommerce-fe/vite.config.ts .env.example
```

Expected: results support the topology facts in the approved design; note explicitly that Catalog gRPC, Order persistence/outbox, and Inventory persistence/events are not currently wired.

- [ ] **Step 2: Create the catalog with a fixed schema**

Create `docs/steering/service-catalog.md` with these sections:

```markdown
# Service Catalog

## Status Vocabulary

- `implemented`: the current process exposes or uses the capability.
- `configured`: deployment or local configuration provisions the capability.
- `accepted but not wired`: architecture assigns ownership, but the current process does not implement the integration.

## Application Surfaces

| Surface | Ownership | Runtime | Local/container ports | Health | Exposure | Datastore/resources | Inbound | Outbound | Status |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |

## Platform Infrastructure

| Component | Ports | Exposure | Consumers | Status |
| --- | --- | --- | --- | --- |

## Usage
```

Populate the application table with Identity, BFF, Catalog, Inventory, Order, and Frontend. Populate infrastructure with Kong, PostgreSQL, Redis, and RabbitMQ. Every row must state inbound and outbound dependencies, using `None currently` rather than an empty cell.

- [ ] **Step 3: Encode the approved topology without overstating readiness**

Ensure the rows state:

- Identity: Java 21/Spring Boot; local/container `8080`; `/actuator/health`; public through Kong; `identity_db` implemented; PostgreSQL outbound.
- BFF: Go 1.24; local `8081`, container `8080`; `/healthz`; public through Kong; no datastore; frontend/Kong inbound; Catalog gRPC address configured but server unavailable.
- Catalog: Go 1.24; HTTP `8081`; `/healthz`; private/local; `catalog_db`, Redis, and gRPC ownership accepted but not wired.
- Inventory: Go 1.24; HTTP `8082`; `/healthz`; private/local; `inventory_db` and RabbitMQ behavior accepted but not wired.
- Order: Java 21/Spring Boot; `8083`; `/actuator/health`; private/local; `order_db`, idempotency, and outbox accepted but not wired.
- Frontend: React 19/Vite 8/TypeScript; `5173`; public local UI; no datastore; browser inbound; Kong `8000` outbound.
- Kong: proxy `8000`, admin `8001`; routes Identity and BFF only.
- PostgreSQL `5432`, Redis `6379`, RabbitMQ `5672` and management `15672`.

- [ ] **Step 4: Route readers to the catalog**

In `docs/steering/README.md`, add a routing-table entry for topology, ports, exposure, health, datastores, or dependencies pointing to `service-catalog.md`. In root `AGENTS.md`, add `service-catalog.md` to Instruction Routing in the same concise link-list style as existing topics.

- [ ] **Step 5: Correct the local BFF bind address**

Use `apply_patch` to change exactly:

```dotenv
BFF_HTTP_ADDR=:8080
```

to:

```dotenv
BFF_HTTP_ADDR=:8081
```

Do not modify container configuration, which remains on `8080`.

- [ ] **Step 6: Validate catalog completeness and links**

Run:

```powershell
$catalog = Get-Content -Raw docs/steering/service-catalog.md
@('Identity','BFF','Catalog','Inventory','Order','Frontend','Kong','PostgreSQL','Redis','RabbitMQ') | ForEach-Object { if ($catalog -notmatch [regex]::Escape($_)) { throw "Missing catalog entry: $_" } }
@('implemented','configured','accepted but not wired') | ForEach-Object { if ($catalog -notmatch [regex]::Escape($_)) { throw "Missing status: $_" } }
Select-String -Path .env.example -Pattern '^BFF_HTTP_ADDR=:8081$'
Test-Path docs/steering/service-catalog.md
git diff --check
```

Expected: all checks complete without an exception; the environment line matches once; `git diff --check` is clean.

- [ ] **Step 7: Commit the catalog**

```powershell
git add AGENTS.md .env.example docs/steering/README.md docs/steering/service-catalog.md
git commit -m "docs: add operational service catalog"
```

### Task 3: Add Onboarding and Local-development Instructions

**Files:**
- Create: `docs/instructions/README.md`
- Create: `docs/instructions/onboarding.md`
- Create: `docs/instructions/local-development.md`
- Modify: `README.md`
- Modify: `AGENTS.md`

**Interfaces:**
- Consumes: `docs/steering/service-catalog.md`, existing Make targets, `.env.example`, service launchers, and npm scripts.
- Produces: task-routed setup and runtime procedures; root links used by all contributors.

- [ ] **Step 1: Create the instruction index**

Create `docs/instructions/README.md` with a `# Developer Instructions` heading and a compact table mapping these intents to one canonical page: first setup → `onboarding.md`; start/stop applications → `local-development.md`; choose checks → `build-and-test.md`; diagnose local runtime → `operations.md`. Add links to `../steering/README.md` for engineering constraints and `../rules/README.md` for contribution policy. Do not copy command sequences into the index.

- [ ] **Step 2: Write onboarding prerequisites and setup order**

Create `docs/instructions/onboarding.md` with:

1. Exact prerequisite floors from Global Constraints.
2. Tool-version checks: `java -version`, `mvn -version`, `go version`, `node --version`, `npm --version`, `docker compose version`, `buf --version`, `pwsh --version`, and optional `make --version`.
3. Setup: clone, `Copy-Item .env.example .env`, `make identity-keys`, `npm --prefix ecommerce-fe ci`, `buf generate`.
4. Narrow smoke checks: `buf lint`, Go tests, Maven tests, and frontend tests as independent commands.
5. Links to the catalog and local-development guide instead of starting the full system.

Use PowerShell syntax for filesystem/environment examples because the supported local host workflow is Windows.

- [ ] **Step 3: Write the host-application local workflow**

Create `docs/instructions/local-development.md` with these exact operational paths:

```powershell
make infra-up
make identity
make order
pwsh -NoProfile -File scripts/start-bff-local.ps1
$env:HTTP_ADDR=':8081'; go run ./services/catalog-service/cmd/server
$env:HTTP_ADDR=':8082'; go run ./services/inventory-service/cmd/server
npm --prefix ecommerce-fe run dev
make infra-down
```

Explain that commands which run a server use separate terminals; applications run on the Windows host while PostgreSQL, Redis, RabbitMQ, and Kong run in Compose. List direct health URLs for Identity `8080`, BFF/Catalog `8081` as mutually exclusive host processes, Inventory `8082`, Order `8083`, plus frontend `5173`; distinguish these from Kong public URLs on `8000`. State that BFF and Catalog cannot both bind host `8081` simultaneously without an explicit local override, while preserving their canonical configured values.

- [ ] **Step 4: Replace stale root quick-start and verification claims**

Update `README.md` so it:

- links to `docs/instructions/README.md`, `docs/steering/service-catalog.md`, and `docs/rules/README.md`;
- uses `make infra-up`/`make infra-down` and host-run applications rather than claiming `make up` starts the entire system;
- describes `make verify` accurately as the existing foundation check, not a universal all-code test harness;
- keeps the Identity demo and security guidance that remain accurate;
- links detailed commands to the instruction pages rather than duplicating their catalogs.

- [ ] **Step 5: Add the root instruction route**

Add one concise paragraph to root `AGENTS.md` after Instruction Routing: developer setup, local runtime, build/test, and operations procedures live in `docs/instructions/README.md`; contribution policy lives in `docs/rules/README.md`. Do not copy procedure details into `AGENTS.md`.

- [ ] **Step 6: Validate files, links, and required commands**

Run:

```powershell
@('README.md','onboarding.md','local-development.md') | ForEach-Object { if (-not (Test-Path (Join-Path 'docs/instructions' $_))) { throw "Missing instruction file: $_" } }
rg -n "make infra-up|start-bff-local|HTTP_ADDR=':8081'|HTTP_ADDR=':8082'|npm --prefix ecommerce-fe run dev|make infra-down" docs/instructions/local-development.md
rg -n "docs/instructions/README.md|service-catalog.md|docs/rules/README.md" README.md AGENTS.md
git diff --check
```

Expected: all required files and command references exist; whitespace validation passes.

- [ ] **Step 7: Commit the first instruction layer**

```powershell
git add AGENTS.md README.md docs/instructions/README.md docs/instructions/onboarding.md docs/instructions/local-development.md
git commit -m "docs: add developer onboarding instructions"
```

### Task 4: Add Build/Test and Operations Instructions

**Files:**
- Create: `docs/instructions/build-and-test.md`
- Create: `docs/instructions/operations.md`
- Modify: `docs/instructions/README.md` only if a link needs correction

**Interfaces:**
- Consumes: `docs/steering/testing.md`, `docs/steering/delivery.md`, Makefile, Maven reactor, Go modules, npm scripts, and existing verification scripts.
- Produces: direct verification commands consumed by contributors and mirrored by CI; local runtime troubleshooting procedures.

- [ ] **Step 1: Map existing executable checks before documenting them**

Run:

```powershell
Get-Content -Raw pom.xml
Get-Content -Raw ecommerce-fe/package.json
Get-Content -Raw Makefile
Get-ChildItem scripts -File | Select-Object -ExpandProperty Name
```

Expected: every planned command maps to an existing module, npm script, Make target, or PowerShell file.

- [ ] **Step 2: Write verification commands by confidence scope**

Create `docs/instructions/build-and-test.md` with a link to `../steering/testing.md` and four sections:

- Focused: Maven `-pl` service tests, individual Go package tests, and individual Vitest file invocation.
- Surface: `mvn -pl services/identity-service -am verify`, `mvn -pl services/order-service -am verify`, tests and builds for each Go service, and frontend `test`, `lint`, `build`.
- Contracts: `buf lint` then `buf generate`, followed by affected language builds.
- Repository: the exact contract, Maven, Go, and frontend command sequence CI uses.

Document `make verify`, `make verify-identity`, and `make verify-edge-auth` as focused foundation/runtime scripts with their infrastructure prerequisites. Explicitly state that no single universal wrapper replaces choosing the relevant direct checks.

- [ ] **Step 3: Write the local operational runbook**

Create `docs/instructions/operations.md` with:

- health requests using `Invoke-RestMethod` against direct service endpoints and Kong proxy routes;
- `make infra-logs` and targeted `docker compose -f deployments/docker/compose.foundation.yml logs <component>`;
- Kong admin inspection at `http://localhost:8001/services` and `/routes`;
- `make identity-keys` and the rule that generated PEM files remain local-only;
- diagnosis for missing `.env`, missing Identity keys, occupied ports, `host.docker.internal` routing, PostgreSQL readiness, RabbitMQ management `15672`, and Catalog gRPC not being implemented;
- safe `make infra-down`, with a warning that destructive volume removal is outside routine shutdown.

Link migration, secrets, rollout, and observability policy to `../steering/delivery.md` rather than restating it.

- [ ] **Step 4: Validate documented commands against workspace paths**

Run:

```powershell
rg -n "buf lint|buf generate|mvn .*verify|go test|go build|npm --prefix ecommerce-fe|verify-foundation|verify-identity|verify-edge-auth" docs/instructions/build-and-test.md
rg -n "Invoke-RestMethod|infra-logs|localhost:8001/services|identity-keys|host.docker.internal|15672|infra-down" docs/instructions/operations.md
git diff --check
```

Expected: all command families and troubleshooting anchors exist; whitespace validation passes.

- [ ] **Step 5: Commit the remaining instructions**

```powershell
git add docs/instructions/README.md docs/instructions/build-and-test.md docs/instructions/operations.md
git commit -m "docs: add verification and operations guides"
```

### Task 5: Add Contribution Rules and Pull-request Template

**Files:**
- Create: `docs/rules/README.md`
- Create: `docs/rules/commit-convention.md`
- Create: `docs/rules/pull-requests.md`
- Create: `docs/rules/ci.md`
- Create: `.github/pull_request_template.md`

**Interfaces:**
- Consumes: the approved branch/approval/squash workflow and impact categories from `docs/steering/change-impact.md`.
- Produces: canonical contribution policies and exact required check names used by the workflow and branch protection.

- [ ] **Step 1: Create the policy index**

Create `docs/rules/README.md` routing pull-request title/final commit questions to `commit-convention.md`, branch/review/merge questions to `pull-requests.md`, and status-check questions to `ci.md`. Link back to `../steering/README.md` and `../instructions/README.md`; do not repeat their content.

- [ ] **Step 2: Define Conventional pull-request titles**

Create `docs/rules/commit-convention.md` with the exact grammar:

```text
<type>(<optional-scope>)<optional-breaking-marker>: <description>
```

List the allowed types `feat`, `fix`, `docs`, `refactor`, `test`, `build`, `ci`, `chore`, `perf`, `revert`; scope pattern `[a-z0-9]+(?:-[a-z0-9]+)*`; and non-empty description. Include valid examples `feat(order): add checkout validation`, `fix!: reject reused refresh tokens`, and `docs: clarify local startup`; include invalid examples with uppercase type, underscore scope, missing colon-space, and empty description. State that the PR title becomes the squash commit subject and intermediate commits are exempt.

- [ ] **Step 3: Define the pull-request and administrator workflow**

Create `docs/rules/pull-requests.md` with contributor steps: branch from current `main`, keep the branch short-lived, open into `main`, complete impact/evidence fields, obtain one approval, satisfy required checks, and squash merge. Add a separate administrator section specifying:

- require a pull request before merging;
- require one approval and dismiss stale approvals;
- require conversation resolution;
- require the named checks `PR title`, `Contracts`, `Java`, `Go`, and `Frontend`;
- require branch to be up to date if the repository team adopts that setting consistently;
- block force pushes and branch deletion;
- enable squash merge and disable merge commits/rebase merges for this workflow.

State that tracked files document these settings but do not configure GitHub remotely.

- [ ] **Step 4: Define CI responsibilities and failure semantics**

Create `docs/rules/ci.md` with a table mapping the five exact display names to title validation, Buf, Maven, Go, and frontend commands. Document PR-to-`main` and push-to-`main` triggers, superseded-run cancellation, read-only contents permission, absence of secrets/deploy/full Compose, and the rule that a failed or unavailable required check blocks merge rather than being converted to pass.

- [ ] **Step 5: Create the pull-request evidence template**

Create `.github/pull_request_template.md` with these headings and checkboxes:

```markdown
## Outcome and Ownership
## Change Impact
- DIRECT:
- CONTRACT:
- DEPENDENT:
- INFRA:
- TEST:
## Compatibility and Migration
## Verification Evidence
| Command | Outcome |
| --- | --- |
## Review Checklist
- [ ] The owning service/application surface is identified.
- [ ] Service and datastore boundaries remain intact.
- [ ] Contract compatibility and dependent consumers were assessed.
- [ ] Secrets and generated artifacts are handled according to repository rules.
- [ ] Verification evidence covers the affected behavior.
- [ ] Unrelated refactors and files are excluded.
```

- [ ] **Step 6: Validate policy consistency**

Run:

```powershell
rg -n "short-lived|one approval|squash|intermediate|PR title|Contracts|Java|Go|Frontend|force push" docs/rules
rg -n "DIRECT|CONTRACT|DEPENDENT|INFRA|TEST|Verification Evidence" .github/pull_request_template.md
git diff --check
```

Expected: workflow decisions, exact check names, and all five impact categories are present; whitespace validation passes.

- [ ] **Step 7: Commit contribution policy**

```powershell
git add docs/rules .github/pull_request_template.md
git commit -m "docs: define pull request and CI policy"
```

### Task 6: Implement and Exercise the Pull-request-title Validator

**Files:**
- Create: `scripts/validate-pr-title.ps1`
- Modify: `docs/rules/commit-convention.md` to add the local invocation if absent

**Interfaces:**
- Consumes: one mandatory string parameter named `Title`.
- Produces: exit code `0` for a valid title; actionable error text and exit code `1` for invalid input; callable by local PowerShell and GitHub Actions.

- [ ] **Step 1: Demonstrate the validator does not exist yet**

Run:

```powershell
pwsh -NoProfile -File scripts/validate-pr-title.ps1 -Title 'feat(order): add checkout validation'
```

Expected: FAIL because `scripts/validate-pr-title.ps1` does not exist.

- [ ] **Step 2: Implement the minimal standalone validator**

Create `scripts/validate-pr-title.ps1` with this implementation:

```powershell
[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateNotNullOrEmpty()]
    [string]$Title
)

$pattern = '^(feat|fix|docs|refactor|test|build|ci|chore|perf|revert)(\([a-z0-9]+(?:-[a-z0-9]+)*\))?!?: .+$'

if ($Title -notmatch $pattern) {
    Write-Error @"
Invalid pull-request title: $Title
Expected: <type>(<optional-scope>)<optional-breaking-marker>: <description>
Allowed types: feat, fix, docs, refactor, test, build, ci, chore, perf, revert
Examples: feat(order): add checkout validation | fix!: reject reused refresh tokens
"@
    exit 1
}

Write-Host "Valid pull-request title: $Title"
exit 0
```

- [ ] **Step 3: Run the valid-title matrix**

Run:

```powershell
@(
  'feat(order): add checkout validation',
  'fix!: reject reused refresh tokens',
  'docs: clarify local startup',
  'ci(pr-title): enforce title policy'
) | ForEach-Object {
  & pwsh -NoProfile -File scripts/validate-pr-title.ps1 -Title $_
  if ($LASTEXITCODE -ne 0) { throw "Expected valid title: $_" }
}
```

Expected: four success messages and no exception.

- [ ] **Step 4: Run the invalid-title matrix**

Run:

```powershell
@(
  'Feat(order): add checkout validation',
  'feat(order_api): add checkout validation',
  'feat(order) add checkout validation',
  'feat(order):'
) | ForEach-Object {
  & pwsh -NoProfile -File scripts/validate-pr-title.ps1 -Title $_ 2>$null
  if ($LASTEXITCODE -eq 0) { throw "Expected invalid title: $_" }
}
```

Expected: no exception; each child process returns a non-zero exit code.

- [ ] **Step 5: Document the reusable local command**

Add this command to `docs/rules/commit-convention.md`:

```powershell
pwsh -NoProfile -File scripts/validate-pr-title.ps1 -Title 'feat(order): add checkout validation'
```

- [ ] **Step 6: Commit the validator**

```powershell
git add scripts/validate-pr-title.ps1 docs/rules/commit-convention.md
git commit -m "ci: add pull request title validator"
```

### Task 7: Add Repository-native GitHub Actions CI

**Files:**
- Create: `.github/workflows/ci.yml`
- Modify: `docs/rules/ci.md` only if verified action/tool details require clarification

**Interfaces:**
- Consumes: `scripts/validate-pr-title.ps1`, Buf configuration, Maven reactor, three Go modules, and frontend npm scripts.
- Produces: five branch-protection checks with display names `PR title`, `Contracts`, `Java`, `Go`, and `Frontend`.

- [ ] **Step 1: Verify maintained action majors from primary sources**

Check the official GitHub repositories/releases for `actions/checkout`, `actions/setup-java`, `actions/setup-go`, `actions/setup-node`, and `bufbuild/buf-setup-action`. Record the current supported major for each before authoring YAML; do not infer a major from third-party examples. If a current major requires a newer GitHub runner runtime, confirm `ubuntu-latest` supports it.

- [ ] **Step 2: Create workflow-level triggers and safety controls**

Start `.github/workflows/ci.yml` with:

```yaml
name: CI

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

permissions:
  contents: read

concurrency:
  group: ci-${{ github.workflow }}-${{ github.event.pull_request.number || github.ref }}
  cancel-in-progress: true
```

Quote the `on` key only if required by the YAML parser used for validation; GitHub accepts the form above.

- [ ] **Step 3: Add the pull-request-title job**

Add a job with job ID `pr-title`, display name `PR title`, condition `github.event_name == 'pull_request'`, `ubuntu-latest`, official checkout, and:

```yaml
- name: Validate pull-request title
  shell: pwsh
  run: ./scripts/validate-pr-title.ps1 -Title $env:PR_TITLE
  env:
    PR_TITLE: ${{ github.event.pull_request.title }}
```

Passing the title through `env` prevents shell interpolation of title content.

- [ ] **Step 4: Add the contract job**

Add job ID `contracts`, display name `Contracts`, `ubuntu-latest`, checkout, Buf setup, then:

```yaml
- name: Lint protobuf contracts
  run: buf lint
- name: Generate contract modules
  run: buf generate
```

This job checks successful deterministic generation without requiring a clean-tree assertion because generated modules are intentionally tracked according to repository policy and platform line-ending behavior can differ.

- [ ] **Step 5: Add the Java job**

Add job ID `java`, display name `Java`, `ubuntu-latest`, checkout, Java setup with distribution `temurin`, `java-version: '21'`, Maven cache, Buf setup/generation, then:

```yaml
- name: Verify Java services
  run: mvn --batch-mode --no-transfer-progress verify
```

Use the root reactor so generated contract modules and both Java services resolve consistently. Do not start Compose; GitHub-hosted Docker remains available to Testcontainers.

- [ ] **Step 6: Add the Go job**

Add job ID `go`, display name `Go`, `ubuntu-latest`, checkout, Go setup with `go-version: '1.24.x'` and dependency caching enabled, Buf setup/generation, then:

```yaml
- name: Test Go services
  run: go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
- name: Build Go services
  run: go build ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
```

If setup-go cannot infer all three module dependency files, set `cache-dependency-path` to the three exact `go.sum` paths as a YAML block scalar.

- [ ] **Step 7: Add the frontend job**

Add job ID `frontend`, display name `Frontend`, `ubuntu-latest`, checkout, Node setup with `node-version: '22.12.0'`, npm cache and `cache-dependency-path: ecommerce-fe/package-lock.json`, then:

```yaml
- name: Install frontend dependencies
  run: npm ci
  working-directory: ecommerce-fe
- name: Test frontend
  run: npm test -- --run
  working-directory: ecommerce-fe
- name: Lint frontend
  run: npm run lint
  working-directory: ecommerce-fe
- name: Build frontend
  run: npm run build
  working-directory: ecommerce-fe
```

If `npm test` already includes non-watch execution in `package.json`, use the exact existing script without appending a duplicate `--run` and keep docs/CI commands identical.

- [ ] **Step 8: Inspect workflow structure and policy alignment**

Run:

```powershell
rg -n "pull_request:|push:|contents: read|cancel-in-progress: true|name: PR title|name: Contracts|name: Java|name: Go|name: Frontend|buf lint|buf generate|mvn .*verify|go test|go build|npm ci|npm .*test|npm run lint|npm run build" .github/workflows/ci.yml
rg -n "PR title|Contracts|Java|Go|Frontend" docs/rules/ci.md docs/rules/pull-requests.md
git diff --check
```

Expected: triggers, safeguards, five display names, and every command family appear; docs use the same display names.

- [ ] **Step 9: Commit CI**

```powershell
git add .github/workflows/ci.yml docs/rules/ci.md
git commit -m "ci: add repository verification workflow"
```

### Task 8: Run Final Verification and Independent Review

**Files:**
- Inspect: all files changed since the design-spec commit
- Modify: only files with verified defects found during this task

**Interfaces:**
- Consumes: all P0–P3 deliverables.
- Produces: evidence that documentation, validator, direct build commands, and final scope satisfy the approved design.

- [ ] **Step 1: Verify scope and wrapper removal**

Run:

```powershell
git diff --name-status ed1dd9c..HEAD
git ls-files harness/verify.ps1
rg -n "harness/verify\.ps1" AGENTS.md README.md docs/steering docs/instructions docs/rules .github scripts Makefile
git status --short -- harness/reports
```

Expected: only P0–P3 files are listed; wrapper/live references are absent; historical reports have not been added or deleted.

- [ ] **Step 2: Validate Markdown links and required content**

Run a PowerShell link check over changed Markdown files: extract relative Markdown targets, ignore `http`, `https`, and in-page `#` anchors, resolve each target relative to its containing file, and throw when the resolved path does not exist. Then rerun the catalog and policy presence checks from Tasks 2 and 5.

Expected: no broken local links and no missing service/status/policy fields.

- [ ] **Step 3: Re-run the title-validator matrices**

Repeat Task 6 Steps 3 and 4.

Expected: all valid titles exit `0`; all invalid titles exit non-zero.

- [ ] **Step 4: Verify contracts**

Run:

```powershell
buf lint
buf generate
```

Expected: both commands exit `0`.

- [ ] **Step 5: Verify all Go services**

Run:

```powershell
go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
go build ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
```

Expected: all packages test and build successfully.

- [ ] **Step 6: Verify both Java services and reactor dependencies**

Run:

```powershell
mvn --batch-mode --no-transfer-progress verify
```

Expected: reactor build succeeds, including Identity and Order tests. If Docker/Testcontainers is unavailable, record the exact environmental failure; do not report it as passing.

- [ ] **Step 7: Verify the frontend from the lockfile**

Run:

```powershell
npm --prefix ecommerce-fe ci
npm --prefix ecommerce-fe test -- --run
npm --prefix ecommerce-fe run lint
npm --prefix ecommerce-fe run build
```

Expected: install, tests, lint, and production build exit `0`. Match the test invocation to the verified package script as described in Task 7.

- [ ] **Step 8: Run hygiene checks**

Run:

```powershell
git diff --check ed1dd9c..HEAD
rg -n "caller_context_received|P4 artifact|slash-command template|AI prompt template" AGENTS.md README.md docs/steering docs/instructions docs/rules .github scripts
git status --short
```

Expected: diff check is clean; no removed logging-test requirement or P4 artifact exists. Legitimate pre-existing source-code markers outside the changed documentation scope are not part of this scan.

- [ ] **Step 9: Request independent review**

Use the `requesting-code-review` skill. Ask the reviewer to compare `ed1dd9c..HEAD` against `docs/superpowers/specs/2026-08-11-developer-harness-and-ci-design.md` and this plan, focusing on topology accuracy, progressive disclosure, validator edge cases, CI command correctness, and accidental scope expansion.

Expected: reviewer either approves or reports concrete findings with file/line evidence.

- [ ] **Step 10: Resolve findings and rerun affected checks**

For each confirmed finding, make the smallest correction with `apply_patch`, rerun its task-level check plus `git diff --check`, and commit it using a scoped Conventional Commit subject. Do not change files solely for reviewer preference when they already meet the spec.

- [ ] **Step 11: Record final repository state**

Run:

```powershell
git status --short --branch
git log --oneline ed1dd9c..HEAD
```

Expected: the working tree is clean, commits are ordered by the delivery sequence, and no merge or push has occurred.
