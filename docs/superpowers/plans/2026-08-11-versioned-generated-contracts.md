# Versioned Generated Contracts Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Track generated Java and Go protobuf contracts and enforce formatting, compatibility, and synchronization locally and in CI.

**Architecture:** Protobuf definitions remain canonical while their generated Java and Go source is committed as reviewed contract output. The Contracts CI job verifies format, lint, compatibility against the exact PR base commit, and that generation makes no Git change.

**Tech Stack:** Buf CLI, Git, GitHub Actions, Java 21/Maven, Go 1.24, Markdown.

## Global Constraints

- Generated protobuf Java and Go source is tracked but never edited by hand.
- Only `buf generate` creates generated contract output.
- Do not modify protobuf schemas, service behavior, public APIs, or deployment configuration.
- Preserve unrelated working-tree changes in `contracts/README.md` and `contracts/openapi.yaml`.
- CI remains read-only and must not commit or push output.
- PR compatibility compares against the exact pull-request base SHA.
- Local and CI commands express the same format, lint, breaking, generation, and synchronization invariants.

---

### Task 1: Version Generated Java and Go Contracts

**Files:**
- Modify: `.gitignore`
- Modify: `docs/steering/api-contracts.md`
- Add: outputs of `buf generate` under `contracts/go/` and `contracts/java/src/main/generated/`

**Interfaces:**
- Consumes: `contracts/proto/`, `buf.yaml`, and `buf.gen.yaml`.
- Produces: tracked generated contract source reproducible by `buf generate`.

- [ ] **Step 1: Verify output directories and user-change boundary**

Run:

```powershell
Get-Content -Raw buf.gen.yaml
rg -n 'contracts/go/\*\*/\*\.pb\.go|contracts/java/src/main/generated' .gitignore
git status --short
```

Expected: only the two generated-output rules are candidates for removal; do not stage the user changes in `contracts/README.md` or `contracts/openapi.yaml`.

- [ ] **Step 2: Remove only the generated-output ignore rules**

Use `apply_patch` to delete:

```gitignore
contracts/go/**/*.pb.go
contracts/java/src/main/generated/
```

Keep all environment, key, build-output, editor, and harness ignore rules.

- [ ] **Step 3: Make contract policy match tracked output**

In `docs/steering/api-contracts.md`, replace ignore-policy statements with this rule:

```markdown
Generated Java and Go contract source is tracked. Do not edit generated files by hand.
Change the protobuf source definition, run `buf format --diff --exit-code`,
`buf lint`, and `buf generate`, then commit the resulting generated output
with the source-contract change.
```

Update the protobuf section and contract checklist to require a clean generated-output diff.

- [ ] **Step 4: Generate and inspect the output**

Run:

```powershell
buf format --diff --exit-code
buf lint
buf generate
git status --short -- contracts/go contracts/java/src/main/generated
```

Expected: generated Java and Go files are visible as Git additions or changes.

- [ ] **Step 5: Prove deterministic generated output**

Run:

```powershell
buf generate
git diff --exit-code -- contracts/go contracts/java/src/main/generated
$untracked = git ls-files --others --exclude-standard -- contracts/go contracts/java/src/main/generated
if ($untracked) { throw "Untracked generated output: $($untracked -join ', ')" }
```

Expected: regeneration changes no tracked file and leaves no untracked generated output.

- [ ] **Step 6: Commit policy and generated outputs**

Stage only `.gitignore`, `docs/steering/api-contracts.md`, `contracts/go`, and `contracts/java/src/main/generated`. Confirm the staged names before committing.

```powershell
git diff --cached --name-only
git commit -m 'build(contracts): version generated clients'
```

Expected: neither unrelated user change is committed.

### Task 2: Enforce Contracts in GitHub Actions

**Files:**
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Consumes: tracked generated outputs and `github.event.pull_request.base.sha`.
- Produces: named diagnostics for formatting, compatibility, and synchronization failures in the `Contracts` check.

- [ ] **Step 1: Verify the compatibility command locally**

Run:

```powershell
$base = git rev-parse HEAD^
buf breaking --against ".git#format=git,commit=$base"
```

Expected: exit code `0` when the current schema is compatible with its baseline.

- [ ] **Step 2: Add format and PR-base breaking steps**

In the `contracts` job, add after Buf setup:

```yaml
- name: Check protobuf formatting
  run: buf format --diff --exit-code
- name: Check protobuf compatibility
  if: github.event_name == 'pull_request'
  env:
    BASE_SHA: ${{ github.event.pull_request.base.sha }}
  run: buf breaking --against ".git#format=git,commit=$BASE_SHA"
```

Keep compatibility limited to pull requests.

- [ ] **Step 3: Add the generation-synchronization step**

After `buf generate`, add:

```yaml
- name: Verify generated contract modules are synchronized
  shell: bash
  run: |
    git diff --exit-code -- contracts/go contracts/java/src/main/generated
    test -z "$(git ls-files --others --exclude-standard -- contracts/go contracts/java/src/main/generated)"
```

The first check catches tracked additions, changes, and deletions; the second catches newly generated untracked files.

- [ ] **Step 4: Validate workflow syntax and diagnostic steps**

Run:

```powershell
python -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml', encoding='utf-8')); print('workflow-yaml-ok')"
rg -n 'Check protobuf formatting|Check protobuf compatibility|BASE_SHA|Verify generated contract modules are synchronized|buf format|buf breaking|git diff --exit-code' .github/workflows/ci.yml
```

Expected: YAML parses and every contract invariant has a named step.

- [ ] **Step 5: Commit CI enforcement**

```powershell
git add .github/workflows/ci.yml
git commit -m 'ci(contracts): enforce compatibility and generation sync'
```

### Task 3: Align Local Procedures and Verify

**Files:**
- Modify: `docs/instructions/build-and-test.md`
- Modify: `docs/rules/ci.md`

**Interfaces:**
- Consumes: the exact contract gates from Task 2.
- Produces: local guidance that matches CI and documented Contract-check responsibility.

- [ ] **Step 1: Update local contract commands**

Replace the Contract checks block in `docs/instructions/build-and-test.md` with:

```powershell
buf format --diff --exit-code
buf lint
buf breaking --against '.git#branch=main'
buf generate
git diff --exit-code -- contracts/go contracts/java/src/main/generated
```

Add the untracked-output diagnostic command:

```powershell
git ls-files --others --exclude-standard -- contracts/go contracts/java/src/main/generated
```

State that feature branches must fetch the current `main` baseline before the local breaking check.

- [ ] **Step 2: Update CI policy**

Change the Contracts row in `docs/rules/ci.md` to include Buf formatting, linting, PR-base compatibility, generation, and tracked-output synchronization.

- [ ] **Step 3: Run end-to-end contract and application checks**

Run:

```powershell
buf format --diff --exit-code
buf lint
$base = git rev-parse HEAD^
buf breaking --against ".git#format=git,commit=$base"
buf generate
git diff --exit-code -- contracts/go contracts/java/src/main/generated
go vet ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
go build ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
npm --prefix ecommerce-fe test
npm --prefix ecommerce-fe run lint
npm --prefix ecommerce-fe run build
```

Expected: all commands pass. Run Maven verification only when Docker is available to Testcontainers; report a missing Docker daemon as an environmental blocker.

- [ ] **Step 4: Check scope and preserve user work**

Run:

```powershell
git diff --check
git status --short
git diff -- contracts/README.md contracts/openapi.yaml
```

Expected: no whitespace errors; the unrelated README and OpenAPI changes remain uncommitted.

- [ ] **Step 5: Commit procedures**

```powershell
git add docs/instructions/build-and-test.md docs/rules/ci.md
git commit -m 'docs(contracts): document generated artifact verification'
```
