# Generic Fullstack Feature Harness Design

## Objective

Provide a reusable AI-driven engineering harness for brownfield repositories. The harness must adapt to repository-local languages, frameworks, architecture, contracts, tests, and build commands without embedding project-specific conventions.

The system coordinates feature definition, interface stabilization, tests, implementation, deterministic verification, bounded self-healing, and evidence-based reporting. It supports backend-only, frontend-only, single-service, and multi-service features.

## Design Principles

- Keep workflow policy in the skill and technology policy in repository-local instructions.
- Discover implementation surfaces before choosing agents, contracts, tests, or commands.
- Treat shared interfaces as typed boundaries, not as a synonym for one OpenAPI file.
- Use deterministic scripts for pass/fail decisions and AI judgment for discovery, planning, classification, and routing.
- Prefer single-agent execution for tightly coupled work. Parallelize only independent units whose shared interfaces are stable.
- Preserve test and contract integrity during self-healing.
- Report machine evidence without converting skipped, blocked, missing, or pre-existing failures into passes.

## Architecture

```text
Requirement
    |
    v
Repository discovery + local instructions
    |
    v
Affected-surface map + dependency graph
    |
    v
Feature specification + acceptance criteria
    |
    v
Changed shared interfaces, when required
    |
    v
Behavior tests
    |
    v
Implementation units ordered by dependencies
    |
    v
Deterministic gates + bounded self-heal
    |
    v
Acceptance-criteria evidence + report
```

The reusable core is split into three layers:

1. The skill defines orchestration, decision rules, ownership, risk checkpoints, and reporting semantics.
2. Discovery and verification scripts resolve repository structure and execute gates.
3. Repository-local instructions and an optional manifest supply conventions or overrides that cannot be inferred safely.

## Components

```text
.agents/skills/fullstack-feature/
|-- SKILL.md
|-- references/
|   |-- repository-discovery.md
|   |-- execution-strategy.md
|   |-- role-contracts.md
|   |-- self-heal-protocol.md
|   `-- reporting.md
`-- scripts/
    |-- discover-project.ps1
    `-- verify.ps1

.agents/agent/
|-- feature-architect.md
|-- feature-qa.md
|-- surface-dev.md
`-- vision-qa.md

harness/
|-- verify.ps1
|-- project.json
`-- reports/
```

`harness/project.json` is optional. The repository wrapper delegates to the reusable verification engine in the skill. A repository may omit the manifest when discovery is unambiguous.

### Skill Orchestrator

`SKILL.md` contains only the essential workflow and decision gates. Detailed discovery, execution, role, self-heal, and reporting guidance is loaded from references when its phase is reached.

The orchestrator may execute inline or delegate roles. Multi-agent execution is permitted only when at least two implementation units can proceed independently after shared interfaces stabilize.

### Repository Discovery

`discover-project.ps1` detects candidate surfaces and emits a machine-readable discovery result. Detection includes:

- Maven and Gradle projects;
- Go modules;
- Node package roots and package-manager lockfiles;
- frontend and backend role hints from structure and scripts;
- OpenAPI, protobuf, AsyncAPI, and other schema locations;
- repository and service-local instruction files;
- test, lint, build, integration, and end-to-end command candidates.

Each surface record contains an identifier, path, role, stack markers, build system, resolved checks, contract relationships, dependency hints, instruction sources, and unresolved ambiguities.

Discovery precedence is:

1. Repository-local instructions.
2. Explicit `harness/project.json` overrides.
3. Safe build-marker and package-script detection.

When evidence is conflicting or incomplete, discovery returns `blocked` with candidates and reasons. It does not guess.

### Optional Project Manifest

The manifest describes only values that need deterministic override. Commands use structured executable, arguments, working directory, environment, and timeout fields. The verifier must not pass manifest text to `Invoke-Expression` or an equivalent shell evaluator.

The manifest may define:

- surface identifiers and paths;
- role and dependency overrides;
- contract locations and consumers/providers;
- verification commands;
- integration and end-to-end environment requirements;
- explicit allowlists for shared resources.

### Verification Engine

The engine supports `discovery`, `contract`, `surface`, `integration`, `e2e`, and `all` stages, with optional surface filtering. It records the resolved command, exit code, duration, log path, dependency context, and one of four statuses:

- `pass`: the configured check ran and succeeded;
- `fail`: the configured check ran and failed;
- `skipped`: no applicable check exists by design;
- `blocked`: a required tool, configuration, environment, or safe resource is unavailable.

`skipped` and `blocked` never count as pass. The engine writes stable JSON reports under `harness/reports/`, including `latest.json` and per-test results when the underlying runner exposes them.

### Roles

- `feature-architect` owns the feature specification, acceptance criteria, affected-surface map, dependency graph, and changed interface contracts.
- `feature-qa` owns acceptance, integration, and end-to-end tests plus the test plan. The chosen test framework follows repository-local test strategy and is independent of backend language.
- `surface-dev` implements one assigned surface according to its local instructions, tests, and interface obligations. Multiple instances may target different services or clients.
- `vision-qa` reviews visual evidence only for user-facing visual changes. Its verdict is advisory.

Role guidance does not name project frameworks, response envelopes, persistence libraries, dependency injection tools, directories, or build commands.

## Feature Data Flow

### 1. Discover and Baseline

Resolve repository rules, surfaces, contracts, commands, and dependencies. Run a scoped baseline for affected surfaces once they are known. Record pre-existing failures separately from regressions introduced by the feature.

### 2. Specify Behavior

Normalize the requirement into acceptance criteria and identify affected surfaces. Record assumptions and derived technical requirements. Do not invent global API or architecture conventions.

### 3. Stabilize Changed Interfaces

Update a shared interface when the feature changes a public or cross-surface boundary:

- REST uses the repository's OpenAPI location when OpenAPI is the established contract.
- gRPC uses protobuf definitions.
- event-driven boundaries use the repository's event schema or AsyncAPI convention.
- internal-only behavior does not receive an artificial external contract.

Shared interfaces must stabilize before dependent implementation units run in parallel. Public interfaces must remain documented even during single-agent execution.

### 4. Define Tests

Derive tests from acceptance criteria, interfaces, and repository test strategy. In multi-agent mode, QA remains independent from implementation agents. In inline mode, the primary agent preserves the same ownership boundaries and test-first discipline.

### 5. Implement by Surface

Create implementation units from the dependency graph. A feature may legitimately modify multiple backend services and clients. Each unit reads the target's local instructions, follows its established architecture, and runs its scoped checks before handoff.

### 6. Verify and Self-Heal

Run scoped gates first, then the complete affected dependency path. Route each failure to one evidence-backed owner. A changed contract invalidates dependent tests and implementation surfaces, which must reconcile before verification resumes.

### 7. Report Evidence

Map machine test results to acceptance criteria. Store human-readable feature artifacts under the repository's established documentation location, defaulting to `docs/features/<slug>/`. Store machine results under `harness/reports/`.

An acceptance criterion is achieved only when linked machine results pass. Missing tests are coverage gaps. Skipped, blocked, and pre-existing failures remain visible with their original status.

## Ownership and Integrity Rules

- An implementation agent may not change shared contracts or acceptance tests to make its gate pass.
- QA may not change application code.
- The architect arbitrates contradictions between behavior, contracts, and tests.
- Harness failures are owned by the primary agent or harness maintainer, not an application surface.
- Every delegated prompt identifies the target surface, allowed paths, instruction sources, interface obligations, and exact scoped verification command.
- Error excerpts sent during self-heal are limited to the relevant section, normally no more than 100 lines.

## Risk Checkpoints

Explicit user approval is required before proceeding with assumptions or derived changes involving:

- authentication, authorization, security boundaries, or secrets;
- database migrations, destructive data changes, or hard-to-recover state;
- shared, staging, or production resources;
- external side effects or third-party integrations;
- money, prices, limits, billing, or settlement;
- privacy, regulated data, or compliance;
- breaking changes to public interfaces.

Autonomous execution cannot approve these decisions.

## Error Routing and Self-Heal

| Failure class | Owner |
|---|---|
| Discovery ambiguity, missing tool, or environment | Primary orchestrator |
| Contract or schema lint | Feature architect |
| Surface unit, lint, or build check | Assigned surface developer |
| Test code, fixture, or test setup | Feature QA |
| Provider violates an interface | Provider surface developer |
| Consumer mishandles an interface | Consumer surface developer |
| Expected behavior conflicts with specification or contract | Feature architect |
| Harness engine defect | Primary orchestrator or harness maintainer |

Self-healing is limited to three rounds per gate and two specification/contract arbitration rounds per feature. Environment-blocked attempts do not consume a developer's retry quota. A timeout terminates child processes and records `blocked` or `fail` according to whether execution began successfully.

The pipeline stops when a quota is exhausted and reports attempted changes, evidence, likely root causes, and next options. It never removes tests, weakens assertions, changes contracts to match an incorrect implementation, or converts a result to skipped to obtain a green report.

The harness does not access shared or production databases or external systems without an explicit allowlist. Ephemeral or isolated resources are the default for integration and end-to-end verification.

## Execution Strategy

Use inline execution when the feature affects one surface, implementation units share unstable decisions, or delegation overhead exceeds isolation benefits.

Use multi-agent execution when at least two implementation units are independent, their shared interfaces are stable, ownership paths do not overlap, and verification can be scoped per unit. Multiple services of different stacks receive separate `surface-dev` assignments rather than one technology-specific backend assignment.

## Verification of the Harness

### Script Tests

Use fixture repositories to cover Maven, Gradle, Go, npm, pnpm, mixed monorepos, OpenAPI, protobuf, and event schemas. Verify instruction and manifest precedence, ambiguity handling, structured command execution, surface filtering, timeout cleanup, dependency invalidation, exit-code propagation, and report schema.

Run discovery against this repository and require detection of both Java and Go services, the Vite client, OpenAPI, and protobuf contracts. Resolve application commands in dry-run mode before executing safe scoped checks.

### Skill Evaluations

Evaluate the workflow on at least these scenarios:

1. A Java, Go, and React microservice feature crossing REST and gRPC boundaries.
2. A Python and Next.js feature with an event contract.
3. A backend-only feature that needs neither a frontend unit nor a new shared contract.

Record baseline behavior without the skill, then repeat with the skill. The revised skill passes when agents identify the correct surfaces and contracts, avoid stack and layout assumptions, choose a valid execution strategy, preserve risk checkpoints, and produce executable verification plans.

### Completion Criteria

- Skill metadata validates and contains trigger conditions without workflow shortcuts.
- No reusable skill or role guidance contains project-specific technology or architecture rules.
- Script fixtures pass.
- Real repository discovery identifies the expected mixed-stack topology.
- Forward evaluations satisfy the behavior criteria.
- Final reporting preserves machine statuses and identifies every acceptance-criteria coverage gap.
