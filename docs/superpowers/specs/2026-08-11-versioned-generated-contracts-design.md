# Versioned Generated Contracts Design

## Goal

Version generated Java and Go protobuf contracts alongside their `.proto`
sources and enforce synchronization in continuous integration.

## Decision

Generated protobuf Java and Go source is a reviewed contract artifact. The
protobuf definitions remain canonical, and generated output must be reproduced
by `buf generate` without changing the working tree.

## Changes

- Remove the generated Java and Go ignore rules from `.gitignore`.
- Update contract steering to state that generated Java and Go source is tracked
  and must not be edited by hand.
- Generate outputs using `buf generate` and commit the resulting Java and Go
  files with their source-contract change.
- Extend the Contracts CI job: run `buf lint`, enforce protobuf formatting with
  `buf format --diff --exit-code`, run `buf generate`, then run
  `git diff --exit-code` to reject stale generated output.
- Update developer verification instructions so local contract verification uses
  the same format, generation, and synchronization checks as CI.

## Boundaries

- Do not commit environment files, build output, or files outside generated
  protobuf Java and Go source.
- Do not alter protobuf schemas, service behavior, APIs, or deployment logic.
- Preserve unrelated working-tree changes, including `contracts/README.md`.

## Verification

- `buf format --diff --exit-code`
- `buf lint`
- `buf generate`
- `git diff --exit-code` after generation, scoped to tracked generated output
- Go tests/builds and Maven verification when Docker/Testcontainers is available

