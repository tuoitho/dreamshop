# Generic Fullstack Feature Harness Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the project-specific fullstack feature pipeline with a reusable, mixed-stack engineering harness that discovers repository evidence, executes deterministic gates, and supports inline or delegated role execution.

**Architecture:** Keep orchestration and ownership policy in a compact skill, mechanical discovery and verification in one canonical PowerShell engine, and project-specific facts in local instructions or an optional JSON manifest. The repository `harness/verify.ps1` remains a logic-free entrypoint. Role prompts are stack-neutral and may be performed inline or delegated.

**Tech Stack:** PowerShell 7, JSON, Markdown agent skills, Python skill validator, plain PowerShell fixture tests.

---

## File Map

- `.agents/skills/fullstack-feature/SKILL.md`: concise generic workflow and reference routing.
- `.agents/skills/fullstack-feature/references/repository-discovery.md`: authority rules, evidence model, and optional manifest contract.
- `.agents/skills/fullstack-feature/references/execution-strategy.md`: inline/delegated selection and dependency ordering.
- `.agents/skills/fullstack-feature/references/role-contracts.md`: ownership contracts independent of agent count.
- `.agents/skills/fullstack-feature/references/self-heal-protocol.md`: failure classification, routing, invalidation, and retry limits.
- `.agents/skills/fullstack-feature/references/reporting.md`: machine status and AC evidence rules.
- `.agents/skills/fullstack-feature/scripts/discover-project.ps1`: mechanical evidence collector.
- `.agents/skills/fullstack-feature/scripts/verify.ps1`: canonical verification engine.
- `.agents/skills/fullstack-feature/tests/*.tests.ps1`: dependency-free script tests.
- `.agents/skills/fullstack-feature/tests/fixtures/*`: representative repository markers and manifests.
- `.agents/skills/fullstack-feature/agents/openai.yaml`: skill UI metadata.
- `.agents/agent/feature-architect.md`: generic architecture role.
- `.agents/agent/feature-qa.md`: generic QA role.
- `.agents/agent/surface-dev.md`: generic implementation-surface role.
- `.agents/agent/vision-qa.md`: generic advisory visual role.
- `harness/verify.ps1`: thin repository wrapper.
- `docs/superpowers/evals/2026-08-10-fullstack-feature-skill-eval.md`: baseline and forward-evaluation evidence.

### Task 1: Record RED Baseline Skill Evaluations

**Files:**
- Create: `docs/superpowers/evals/2026-08-10-fullstack-feature-skill-eval.md`

- [ ] **Step 1: Run three fresh-context scenarios without loading the skill**

Dispatch independent read-only evaluators with these task shapes:

```text
1. Plan a stock-reservation feature spanning a Maven order service, Go inventory
   service, React client, REST API, and protobuf boundary.
2. Plan an account-event feature spanning a web application and Python worker
   with an event schema.
3. Plan a backend-only internal calculation change with no public interface change.
```

Require each evaluator to report affected surfaces, interfaces, execution mode, role assignments, and verification commands. Do not reveal the intended design or current skill.

- [ ] **Step 2: Capture baseline failures verbatim**

Create the evaluation document with this stable shape:

```markdown
# Fullstack Feature Skill Evaluation

## Baseline

### Scenario 1
- Raw decision: Copy the evaluator response verbatim.
- Failure pattern: Classify only behavior demonstrated by that response.

## Forward Evaluation

Pending until the revised skill exists.
```

Record exact rationalizations that force one backend agent, one OpenAPI file, frontend work for backend-only scope, or mandatory delegation.

- [ ] **Step 3: Commit the baseline evidence**

```powershell
git add -- docs/superpowers/evals/2026-08-10-fullstack-feature-skill-eval.md
git commit -m "test: record fullstack skill baseline"
```

### Task 2: Add Failing Mechanical Discovery Tests

**Files:**
- Create: `.agents/skills/fullstack-feature/tests/discover-project.tests.ps1`
- Create: `.agents/skills/fullstack-feature/tests/fixtures/java-go-react-microservices/AGENTS.md`
- Create: `.agents/skills/fullstack-feature/tests/fixtures/java-go-react-microservices/services/order/pom.xml`
- Create: `.agents/skills/fullstack-feature/tests/fixtures/java-go-react-microservices/services/inventory/go.mod`
- Create: `.agents/skills/fullstack-feature/tests/fixtures/java-go-react-microservices/web/package.json`
- Create: `.agents/skills/fullstack-feature/tests/fixtures/java-go-react-microservices/web/package-lock.json`
- Create: `.agents/skills/fullstack-feature/tests/fixtures/java-go-react-microservices/contracts/openapi/order.yaml`
- Create: `.agents/skills/fullstack-feature/tests/fixtures/java-go-react-microservices/contracts/proto/inventory.proto`
- Create: `.agents/skills/fullstack-feature/tests/fixtures/ambiguous-node/package.json`
- Create: `.agents/skills/fullstack-feature/tests/fixtures/ambiguous-node/package-lock.json`
- Create: `.agents/skills/fullstack-feature/tests/fixtures/ambiguous-node/pnpm-lock.yaml`

- [ ] **Step 1: Write the discovery contract tests**

Use a dependency-free assertion helper and invoke the missing script:

```powershell
$ErrorActionPreference = 'Stop'
$skillRoot = Split-Path $PSScriptRoot -Parent
$discover = Join-Path $skillRoot 'scripts/discover-project.ps1'

function Assert-Equal($Actual, $Expected, [string]$Message) {
    if ($Actual -ne $Expected) {
        throw "$Message. Expected '$Expected', got '$Actual'."
    }
}

$fixture = Join-Path $PSScriptRoot 'fixtures/java-go-react-microservices'
$result = & $discover -RepoRoot $fixture | ConvertFrom-Json
Assert-Equal $result.schemaVersion 1 'schema version'
Assert-Equal @($result.candidates).Count 3 'candidate count'
Assert-Equal @($result.contracts).Count 2 'contract evidence count'
if ($result.candidates[0].PSObject.Properties.Name -contains 'role') {
    throw 'Mechanical discovery must not infer semantic roles.'
}
```

Add assertions for instruction discovery, Maven/Go/Node markers, structured command candidates, absence of `role`, `ownership`, and `dependsOn`, plus `blocked` ambiguity for conflicting Node lockfiles.

- [ ] **Step 2: Run the test and verify RED**

Run:

```powershell
pwsh -NoProfile -File .agents/skills/fullstack-feature/tests/discover-project.tests.ps1
```

Expected: FAIL because `scripts/discover-project.ps1` does not exist.

- [ ] **Step 3: Commit the RED tests and fixtures**

```powershell
git add -- .agents/skills/fullstack-feature/tests
git commit -m "test: define repository discovery contract"
```

### Task 3: Implement Mechanical Repository Discovery

**Files:**
- Create: `.agents/skills/fullstack-feature/scripts/discover-project.ps1`
- Test: `.agents/skills/fullstack-feature/tests/discover-project.tests.ps1`

- [ ] **Step 1: Implement the evidence collector**

Expose this interface:

```powershell
#Requires -Version 7
[CmdletBinding()]
param(
    [Parameter()][string]$RepoRoot = (Get-Location).Path,
    [Parameter()][string]$ManifestPath,
    [Parameter()][string]$OutputPath
)
```

Return one JSON document with:

```json
{
  "schemaVersion": 1,
  "repoRoot": "C:/fixtures/java-go-react-microservices",
  "status": "pass|blocked",
  "instructionFiles": [],
  "contracts": [],
  "candidates": [],
  "ambiguities": []
}
```

Use marker definitions for Maven, Gradle, Go, and Node. Exclude `.git`, dependency caches, build outputs, and `harness/reports`. Emit only filesystem markers, instruction files, nearby contract files, package scripts, command candidates, and ambiguity evidence. Parse the optional manifest only to override machine facts; never infer semantic roles or dependency relationships from proximity.

- [ ] **Step 2: Run discovery tests to GREEN**

Run:

```powershell
pwsh -NoProfile -File .agents/skills/fullstack-feature/tests/discover-project.tests.ps1
```

Expected: PASS with a summary naming all discovery assertions.

- [ ] **Step 3: Run real-repository discovery in dry evidence mode**

Run:

```powershell
pwsh -NoProfile -File .agents/skills/fullstack-feature/scripts/discover-project.ps1 -RepoRoot .
```

Expected: valid JSON containing the repository's Maven, Go, and Node candidates plus OpenAPI/protobuf evidence, without semantic role or dependency fields.

- [ ] **Step 4: Commit discovery implementation**

```powershell
git add -- .agents/skills/fullstack-feature/scripts/discover-project.ps1
git commit -m "feat: add mechanical repository discovery"
```

### Task 4: Add Failing Verification Engine Tests

**Files:**
- Create: `.agents/skills/fullstack-feature/tests/verify.tests.ps1`
- Create: `.agents/skills/fullstack-feature/tests/fixtures/verification/project.json`
- Create: `.agents/skills/fullstack-feature/tests/fixtures/verification/go.mod`
- Create: `.agents/skills/fullstack-feature/tests/fixtures/verification/test-results.json`

- [ ] **Step 1: Define structured-command test cases**

The fixture manifest must use arrays rather than shell strings:

```json
{
  "schemaVersion": 1,
  "surfaces": [
    {
      "id": "sample",
      "path": ".",
      "checks": {
        "test": {
          "executable": "pwsh",
          "arguments": ["-NoProfile", "-Command", "exit 0"],
          "timeoutSec": 10,
          "testResults": {
            "path": "test-results.json",
            "format": "harness-tests-v1"
          }
        }
      }
    }
  ]
}
```

Use a normalized result fixture with schema version, test name, kind, status, and duration. Test `pass`, non-zero `fail` with exact exit code, `skipped` for an inapplicable stage, `blocked` for a missing executable, timeout cleanup, `-Surface` filtering, `-DryRun`, stable `latest.json` fields, and exact copying into `latest-tests.json`.

- [ ] **Step 2: Run the tests and verify RED**

Run:

```powershell
pwsh -NoProfile -File .agents/skills/fullstack-feature/tests/verify.tests.ps1
```

Expected: FAIL because the canonical verification engine does not exist.

- [ ] **Step 3: Commit the RED verification tests**

```powershell
git add -- .agents/skills/fullstack-feature/tests/verify.tests.ps1 .agents/skills/fullstack-feature/tests/fixtures/verification
git commit -m "test: define verification engine contract"
```

### Task 5: Implement the Canonical Verifier and Thin Wrapper

**Files:**
- Create: `.agents/skills/fullstack-feature/scripts/verify.ps1`
- Replace: `harness/verify.ps1`
- Test: `.agents/skills/fullstack-feature/tests/verify.tests.ps1`

- [ ] **Step 1: Implement the canonical engine**

Expose:

```powershell
#Requires -Version 7
[CmdletBinding()]
param(
    [ValidateSet('discovery', 'contract', 'surface', 'integration', 'e2e', 'all')]
    [string]$Stage = 'all',
    [string[]]$Surface,
    [string]$RepoRoot = (Get-Location).Path,
    [string]$ManifestPath,
    [switch]$DryRun
)
```

Resolve discovery evidence and explicit manifest commands. Execute commands with `System.Diagnostics.Process`, `ProcessStartInfo.ArgumentList`, redirected output, explicit working directory, environment overrides, timeout, and process-tree termination. Never use `Invoke-Expression`.

Write gate records with:

```json
{
  "gateId": "surface:sample:test",
  "surfaceId": "sample",
  "status": "pass|fail|skipped|blocked",
  "exitCode": 0,
  "durationSec": 0.1,
  "logPath": "harness/reports/20260810-120000/surface-sample-test.log",
  "command": { "executable": "pwsh", "arguments": [] }
}
```

Calculate overall status in severity order `fail`, `blocked`, `pass`, `skipped`. Preserve machine statuses exactly.

When a command declares `testResults.format` as `harness-tests-v1`, read its normalized JSON after execution and copy each machine-reported `pass`, `fail`, or `skip` status into the run report and `harness/reports/latest-tests.json`. When no normalized result source is configured, write an empty test list; never infer individual test outcomes from console text.

- [ ] **Step 2: Replace the repository wrapper**

The complete wrapper remains parameter forwarding only:

```powershell
#Requires -Version 7
[CmdletBinding()]
param(
    [ValidateSet('discovery', 'contract', 'surface', 'integration', 'e2e', 'all')]
    [string]$Stage = 'all',
    [string[]]$Surface,
    [string]$ManifestPath,
    [switch]$DryRun
)

$engine = Join-Path (Split-Path $PSScriptRoot -Parent) '.agents/skills/fullstack-feature/scripts/verify.ps1'
& $engine -Stage $Stage -Surface $Surface -RepoRoot (Split-Path $PSScriptRoot -Parent) -ManifestPath $ManifestPath -DryRun:$DryRun
exit $LASTEXITCODE
```

- [ ] **Step 3: Run verification tests to GREEN**

Run:

```powershell
pwsh -NoProfile -File .agents/skills/fullstack-feature/tests/verify.tests.ps1
```

Expected: PASS for structured execution, statuses, timeout, filters, dry-run, and report schema.

- [ ] **Step 4: Verify the real wrapper without running application commands**

Run:

```powershell
pwsh -NoProfile -File harness/verify.ps1 -Stage discovery -DryRun
pwsh -NoProfile -File harness/verify.ps1 -Stage all -DryRun
```

Expected: discovery evidence and a report containing resolved or skipped/blocked checks without executing builds or tests.

- [ ] **Step 5: Commit the engine and wrapper**

```powershell
git add -- .agents/skills/fullstack-feature/scripts/verify.ps1 harness/verify.ps1
git commit -m "feat: add generic verification engine"
```

### Task 6: Replace the Skill with the Generic Workflow

**Files:**
- Replace: `.agents/skills/fullstack-feature/SKILL.md`
- Delete: `.agents/skills/fullstack-feature/references/scaffold.md`
- Create: `.agents/skills/fullstack-feature/references/repository-discovery.md`
- Create: `.agents/skills/fullstack-feature/references/execution-strategy.md`
- Create: `.agents/skills/fullstack-feature/references/role-contracts.md`
- Replace: `.agents/skills/fullstack-feature/references/self-heal-protocol.md`
- Create: `.agents/skills/fullstack-feature/references/reporting.md`

- [ ] **Step 1: Write minimal trigger metadata and workflow**

Use this frontmatter:

```yaml
---
name: fullstack-feature
description: Use when implementing a feature that may span one or more application surfaces in a brownfield repository and needs coordinated specification, testing, implementation, deterministic verification, or evidence across mixed stacks.
---
```

Keep the body focused on discovery, affected surfaces, risk checkpoint, tests, implementation-unit ordering, deterministic verification, self-heal, and evidence. State explicitly that roles are responsibilities rather than mandatory agents and that user execution-mode preferences are followed unless unsafe or unavailable.

- [ ] **Step 2: Write phase-specific references**

Put authority and evidence rules in `repository-discovery.md`; inline/delegated heuristics in `execution-strategy.md`; ownership contracts in `role-contracts.md`; route/retry/invalidation rules in `self-heal-protocol.md`; and machine-status/AC mapping in `reporting.md`. Each reference is linked directly from `SKILL.md` with a precise read condition.

- [ ] **Step 3: Remove the project scaffold reference**

Delete `references/scaffold.md`. No replacement may mention a fixed backend directory, frontend directory, database name, response envelope, framework, or one contract location.

- [ ] **Step 4: Scan for leaked project conventions**

Run:

```powershell
rg -n -i "fico|gin|gorm|wire|react 19|vite|zod|backend/|frontend/|contracts/openapi.yaml|fico_iam_db" .agents/skills/fullstack-feature
```

Expected: no matches outside explicit fixture filenames or fixture contents.

- [ ] **Step 5: Commit the generic skill**

```powershell
git add -- .agents/skills/fullstack-feature
git commit -m "feat: generalize fullstack feature workflow"
```

### Task 7: Replace Project-Specific Agents with Generic Roles

**Files:**
- Delete: `.agents/agent/architect.md`
- Delete: `.agents/agent/qa.md`
- Delete: `.agents/agent/backend-dev.md`
- Delete: `.agents/agent/frontend-dev.md`
- Delete: `.agents/agent/fico-backend.md`
- Delete: `.agents/agent/fico-frontend.md`
- Create: `.agents/agent/feature-architect.md`
- Create: `.agents/agent/feature-qa.md`
- Create: `.agents/agent/surface-dev.md`
- Replace: `.agents/agent/vision-qa.md`

- [ ] **Step 1: Write generic role prompts**

Every role must require target paths, local instruction discovery, allowed writes, interface obligations, and scoped verification. `surface-dev` must infer language, framework, architecture, and test commands from its assigned surface rather than from its name.

- [ ] **Step 2: Preserve ownership integrity**

Include these role invariants:

```text
feature-architect: specification and changed shared interfaces; no application code
feature-qa: acceptance/integration/e2e tests; no application code or contracts
surface-dev: assigned surface and local tests; no acceptance tests or shared contracts
vision-qa: visual report only; no implementation changes
```

- [ ] **Step 3: Delete legacy role prompts and scan**

Run:

```powershell
rg -n -i "fico|gin|gorm|wire|react 19|vite|zod|modular monolith|backend/|frontend/" .agents/agent
```

Expected: no project-specific matches in the four generic roles.

- [ ] **Step 4: Commit role replacement**

```powershell
git add -- .agents/agent
git commit -m "feat: replace stack-specific feature agents"
```

### Task 8: Generate Metadata and Validate the Skill

**Files:**
- Create: `.agents/skills/fullstack-feature/agents/openai.yaml`

- [ ] **Step 1: Generate deterministic UI metadata**

Run the skill-creator generator with:

```text
display_name=Fullstack Feature Harness
short_description=Coordinate mixed-stack feature delivery
default_prompt=Use $fullstack-feature to implement this feature across the affected repository surfaces with deterministic verification.
```

- [ ] **Step 2: Run structural validation**

Run:

```powershell
python C:/Users/TuoiTho/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/fullstack-feature
```

Expected: validation succeeds.

- [ ] **Step 3: Commit metadata**

```powershell
git add -- .agents/skills/fullstack-feature/agents/openai.yaml
git commit -m "chore: add fullstack skill metadata"
```

### Task 9: Forward-Test and Refine the Revised Skill

**Files:**
- Modify if needed: `.agents/skills/fullstack-feature/SKILL.md`
- Modify if needed: `.agents/skills/fullstack-feature/references/*.md`
- Modify: `docs/superpowers/evals/2026-08-10-fullstack-feature-skill-eval.md`

- [ ] **Step 1: Run the three baseline scenarios with the revised skill**

Use fresh evaluators. Give each only the skill path, its scenario, and raw fixture evidence. Do not include baseline diagnoses or expected answers.

- [ ] **Step 2: Evaluate transferable behavior**

Require correct affected surfaces, interface selection, semantic classification outside the script, inline/delegated choice, role ownership, high-risk checkpoint, and executable verification plan. Read every evaluator response manually.

- [ ] **Step 3: Close observed loopholes**

If an evaluator mandates four agents, invents one OpenAPI contract, assigns semantic dependencies to the script, or treats skipped/blocked as pass, add the smallest positive recipe or observable conditional to the relevant reference and rerun that scenario.

- [ ] **Step 4: Record forward results and commit**

```powershell
git add -- .agents/skills/fullstack-feature docs/superpowers/evals/2026-08-10-fullstack-feature-skill-eval.md
git commit -m "test: verify generic fullstack skill behavior"
```

### Task 10: Final Verification and Review

**Files:**
- Verify all files changed by Tasks 1-9.

- [ ] **Step 1: Run all script tests**

```powershell
pwsh -NoProfile -File .agents/skills/fullstack-feature/tests/discover-project.tests.ps1
pwsh -NoProfile -File .agents/skills/fullstack-feature/tests/verify.tests.ps1
```

Expected: both PASS.

- [ ] **Step 2: Re-run skill validation and residue scans**

```powershell
python C:/Users/TuoiTho/.codex/skills/.system/skill-creator/scripts/quick_validate.py .agents/skills/fullstack-feature
rg -n -i "fico|gin|gorm|wire|react 19|zod|fico_iam_db" .agents/skills/fullstack-feature .agents/agent
git diff --check
```

Expected: skill validation succeeds, residue scan finds only intentional fixture data if any, and diff check succeeds.

- [ ] **Step 3: Run real discovery and wrapper dry-runs**

```powershell
pwsh -NoProfile -File .agents/skills/fullstack-feature/scripts/discover-project.ps1 -RepoRoot .
pwsh -NoProfile -File harness/verify.ps1 -Stage discovery -DryRun
pwsh -NoProfile -File harness/verify.ps1 -Stage all -DryRun
```

Expected: valid evidence and reports without application command execution.

- [ ] **Step 4: Request independent code review**

Review scope: discovery does not make semantic claims; commands cannot use shell evaluation; wrapper contains no verification logic; statuses remain exact; role prompts contain no stack rules; unrelated dirty files remain untouched.

- [ ] **Step 5: Inspect final repository state**

```powershell
git status --short
git log --oneline -12
```

Expected: only pre-existing unrelated changes remain unstaged; all harness work is committed in focused commits.
