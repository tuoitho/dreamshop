# CI Policy

CI runs on pull requests targeting `main` and on pushes to `main`. The required
checks are independent so branch protection can identify the failing surface.

| Check | Responsibility |
| --- | --- |
| PR title | Validate the pull-request title with `scripts/validate-pr-title`. |
| Boundaries | Run `scripts/check-boundaries` to reject cross-service source, build, and database dependencies. |
| Contracts | Check Buf formatting, linting, breaking-change compatibility against `main`, generation, and tracked generated-output synchronization. |
| Java | Generate contracts and run Maven `verify` for the reactor. |
| Go | Generate contracts, test all four Go services and `scripts/`, and build all of them. |
| Frontend | Install from the lockfile, test, lint, and build the React application. |

Superseded runs for the same pull request or branch are cancelled. Workflow
permissions grant read-only repository contents. CI does not use secrets, start
the full local Compose stack, call production/external services, publish images,
or deploy.

A failed or unavailable required check blocks merge. CI does not turn blocked,
skipped, or infrastructure-failed verification into a passing result.
