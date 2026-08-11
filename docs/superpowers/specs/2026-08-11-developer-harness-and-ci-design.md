# Developer Harness and CI Design

## Context

The repository has a strong progressive-disclosure instruction hierarchy:

- Root and application-local `AGENTS.md` files define mandatory agent behavior.
- `docs/steering/` contains canonical architecture, boundary, contract, impact,
  testing, and delivery rules.
- `README.md`, `Makefile`, and local scripts provide partial developer workflows.
- `.agents/skills/fullstack-feature/` provides adaptive feature orchestration.

The remaining developer harness is incomplete. `harness/verify.ps1` delegates to
`.agents/skills/fullstack-feature/scripts/verify.ps1`, but that engine is not
present at the current revision. The repository also lacks an operational service
catalog, a focused developer instruction set, repository policy documents, and CI
enforcement.

## Goals

- Remove the broken verification entry point without replacing it with another
  universal wrapper.
- Add an accurate operational service catalog that distinguishes implemented,
  configured, and accepted-but-not-wired behavior.
- Provide progressive-disclosure instructions for onboarding, local development,
  build/test, and common operations.
- Establish a short-lived-branch and pull-request workflow for `main`.
- Enforce contract, Java, Go, frontend, and pull-request-title checks in GitHub
  Actions using repository-native commands.
- Keep steering, instructions, rules, and executable checks as separate concerns
  with clear sources of truth.

## Non-goals

- Restore the deleted generic verification engine or add a harness manifest.
- Add Taskfile, pre-commit hooks, Husky, Lefthook, or commitlint.
- Require every intermediate commit to follow Conventional Commits.
- Add AI prompt or slash-command templates.
- Configure GitHub branch protection through an external API.
- Add deployment automation, image publishing, or automatic staging/production
  rollout.
- Implement service business features, gRPC servers, persistence, caches, or
  event consumers that are not already wired.

## Source-of-truth Model

The documentation layers remain:

1. `AGENTS.md` and local `AGENTS.md` files: mandatory coding-agent instructions.
2. `docs/steering/`: canonical architecture and engineering constraints.
3. `docs/instructions/`: developer procedures that execute those constraints.
4. `docs/rules/`: contribution and CI policies that are mandatory for changes.
5. Build files, scripts, and `.github/workflows/`: executable behavior.

Detailed procedures and policies have one canonical home. Root files link to the
canonical document instead of duplicating it. Service-local instructions remain
authoritative for the narrowest application surface.

## Impact Classification

- `DIRECT`: delete `harness/verify.ps1`; add steering, instruction, rule, CI,
  pull-request-template, and title-validation files.
- `CONTRACT`: no public API, protobuf, event, or REST contract changes.
- `DEPENDENT`: developers, coding agents, pull requests, and GitHub Actions consume
  the new documentation and policy.
- `INFRA`: GitHub Actions configuration and one local BFF environment default.
- `TEST`: documentation/link checks, PR-title validation cases, CI configuration
  inspection, and all repository-native verification commands.

## Phase 0: Remove the Broken Wrapper

Delete the tracked `harness/verify.ps1`. Do not restore the absent engine, add a
manifest, or create a replacement wrapper.

Local `harness/reports/` data is not part of the tracked change. It may contain
historical or user-generated evidence and must not be deleted. Once the wrapper
is removed, the `fullstack-feature` skill follows its existing fallback: use
commands from root/local instructions, Makefile, scripts, and build files when no
canonical harness exists.

Remove or update documentation references that incorrectly claim
`harness/verify.ps1` is an available gate. Historical specs and plans remain
unchanged because they describe prior decisions and implementation history.

## Phase 1: Service Catalog

Create `docs/steering/service-catalog.md` and link it from root `AGENTS.md` and
`docs/steering/README.md`.

The catalog contains one row per backend service plus the frontend and platform
infrastructure. It records:

- Ownership and runtime/language.
- Canonical local and container ports.
- Health endpoint.
- Public, private, or local-only exposure.
- Owned datastore or infrastructure resource.
- Inbound callers and outbound dependencies.
- Current status: `implemented`, `configured`, or `accepted but not wired`.

The status column prevents accepted architecture from being represented as
implemented behavior. For example, Order owns `order_db` and outbox behavior by
architecture, but its current Spring Boot skeleton has no datasource/outbox
wiring. Catalog and Inventory have provisioned databases and accepted ownership,
but their current processes expose only HTTP health endpoints.

### Canonical Topology Facts

- Kong exposes proxy `8000` and admin `8001` locally.
- Identity runs on `8080` through its local launcher/container and exposes
  `/actuator/health`; Kong exposes `/api/v1/auth` and protected `/api/v1` routes.
- BFF uses container port `8080`, local host port `8081`, and `/healthz`; Kong
  exposes `/api/v1/storefront` to the local BFF at `host.docker.internal:8081`.
- Catalog uses HTTP `8081` and `/healthz`. Its gRPC contract/address is accepted
  for internal use, but the current service does not implement a gRPC server.
- Inventory uses HTTP `8082` and `/healthz`; database/event behavior is accepted
  but not wired.
- Order uses `8083` and `/actuator/health`; `order_db`, idempotency, and outbox are
  owned but not wired in the current skeleton.
- Frontend uses Vite `5173` and calls public APIs through Kong at `8000`.
- PostgreSQL uses `5432`, Redis `6379`, RabbitMQ `5672`/management `15672`.

Correct `.env.example` from `BFF_HTTP_ADDR=:8080` to `BFF_HTTP_ADDR=:8081` so the
canonical local launcher matches Kong and does not conflict with Identity.
Container BFF configuration remains `8080`.

## Phase 2: Developer Instructions

Create:

```text
docs/instructions/
  README.md
  onboarding.md
  local-development.md
  build-and-test.md
  operations.md
```

### Instruction Index

`docs/instructions/README.md` routes developers by task and links back to relevant
steering and rules. It contains no duplicated command catalogue.

### Onboarding

`onboarding.md` defines supported prerequisites:

- Java 21 and Maven 3.9 or newer.
- Go 1.24 or newer within the 1.24 line used by the workspace.
- Node.js 22.12 or newer for Vite 8, with npm from the installed Node release.
- Docker Desktop/Compose v2, Buf CLI, PowerShell 7, and GNU Make when using
  Makefile shortcuts.

It documents clone/setup, `.env` creation, local Identity key generation,
dependency installation, contract generation, and narrow smoke checks. It does
not instruct developers to start every application for every task.

### Local Development

`local-development.md` documents:

- Infrastructure startup/shutdown through existing Makefile/Docker Compose
  commands.
- Running Identity and Order through service launchers.
- Running BFF through `scripts/start-bff-local.ps1` with local port `8081`.
- Running Catalog/Inventory directly with their canonical `HTTP_ADDR` values.
- Running the frontend through npm.
- Which public URLs go through Kong and which health URLs are direct/local.
- The rule that application services run on the host during local development.

### Build and Test

`build-and-test.md` routes focused, surface, contract, and repository-level
verification. It links to `docs/steering/testing.md` for policy and records only
commands that exist in the workspace. It includes Maven progressive testing,
Go tests/builds, frontend test/lint/build, Buf lint/generation, and existing
verification scripts.

### Operations

`operations.md` is a local operational runbook covering health checks, logs,
Kong routing, Identity key generation, common configuration failures, port
conflicts, database/broker readiness, and safe shutdown. It links to
`docs/steering/delivery.md` for rollout, migration, secrets, and observability
rules rather than repeating them.

Update root `README.md` and `AGENTS.md` with concise links to the instruction
index. Do not turn either root file into a second onboarding guide.

## Phase 3: Contribution Rules and CI

Create:

```text
docs/rules/
  README.md
  commit-convention.md
  pull-requests.md
  ci.md
.github/
  workflows/ci.yml
  pull_request_template.md
scripts/
  validate-pr-title.ps1
```

### Contribution Workflow

- Create a short-lived feature branch from current `main`.
- Open a pull request into `main`.
- Require at least one approval and all required status checks.
- Use squash merge only.
- Use the pull-request title as the final squash commit subject.
- Do not require every intermediate branch commit to follow Conventional
  Commits.

GitHub repository settings enforce review count, required checks, squash-only
merges, and protection from force-push/deletion. Because these settings are not
fully representable in tracked files, `docs/rules/pull-requests.md` provides the
exact settings an administrator must enable and names the required CI checks.

### Conventional Pull-request Titles

`scripts/validate-pr-title.ps1` validates:

```text
<type>(<optional-scope>)<optional-breaking-marker>: <description>
```

Allowed types are `feat`, `fix`, `docs`, `refactor`, `test`, `build`, `ci`,
`chore`, `perf`, and `revert`. Scope uses lowercase letters, digits, and hyphens.
The description is non-empty. Examples and breaking-change syntax are documented
in `commit-convention.md`.

The validator accepts a title parameter, prints an actionable failure message,
and returns a non-zero exit code for invalid input. It is reusable locally and by
CI without introducing commitlint or a Node dependency at repository root.

### Pull-request Template

The template captures:

- Outcome and owning surface.
- `DIRECT`, `CONTRACT`, `DEPENDENT`, `INFRA`, and `TEST` impact.
- Compatibility/migration notes.
- Verification commands and outcomes.
- Required review checklist for boundaries, secrets, generated files, and
  unrelated changes.

### GitHub Actions

`.github/workflows/ci.yml` runs on pull requests to `main` and pushes to `main`.
It uses maintained official/setup actions whose current major versions are
verified against their primary documentation at implementation time.

Jobs are independently visible for branch protection:

- `pr-title`: run only for pull requests and invoke the PowerShell title
  validator.
- `contracts`: set up Buf, run `buf lint`, and confirm `buf generate` succeeds.
- `java`: set up Java 21 and Buf, generate contracts, then run Maven `verify` for
  Identity and Order with required reactor modules. GitHub-hosted Docker supports
  Testcontainers.
- `go`: set up Go 1.24 and Buf, generate contracts, then test and build BFF,
  Catalog, and Inventory.
- `frontend`: set up Node 22 with npm cache, run `npm ci`, test, lint, and build.

Use concurrency cancellation for superseded runs on the same pull request or
branch. Grant read-only repository contents permission. Avoid secrets and
production/external services. CI does not start the full local Compose stack or
perform deployment.

## Failure Handling

- A failed CI job is a blocking status, not a warning.
- Documentation distinguishes current wiring from accepted architecture so
  missing implementation is not reported as operational readiness.
- Instructions report exact prerequisites and do not hide missing tools behind
  wrappers.
- The PR-title validator explains the accepted format and examples.
- If a test depends on unavailable external infrastructure, the job reports the
  exact failure; CI does not convert blocked/skipped checks into pass.

## Verification

Validate in layers:

1. Confirm `harness/verify.ps1` is no longer tracked and no live documentation
   links to it.
2. Validate every new Markdown link and required service-catalog field.
3. Check catalog ports/routes/dependencies against Dockerfiles, configs, Kong,
   launchers, and Compose.
4. Run the PR-title validator against valid, invalid, scoped, and breaking-title
   examples.
5. Inspect CI triggers, permissions, concurrency, job names, toolchain versions,
   and commands against current primary documentation and repository build files.
6. Run `buf lint` and `buf generate`.
7. Run Go tests and builds for all three Go services.
8. Run Maven `verify` for Identity and Order with reactor dependencies.
9. Run frontend test, lint, and build after a clean `npm ci`-equivalent dependency
   state where practical.
10. Run `git diff --check`, scope checks, and a prompt-residue scan.
11. Request independent review against this design and the implementation plan.

## Delivery Order

1. Remove the broken wrapper.
2. Establish and validate the service catalog, including the local BFF port fix.
3. Add developer instructions against the validated catalog and existing tools.
4. Add contribution rules and the reusable PR-title validator.
5. Add CI that consumes the documented commands/rules.
6. Run repository-wide verification and document the GitHub settings that still
   require administrator configuration.
