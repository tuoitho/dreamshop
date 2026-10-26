# Contributing, Pull Requests, and CI

## Pull-request workflow

1. Create a short-lived feature branch from the current `main`.
2. Keep the change scoped to the owning service, package, or application surface.
3. Open a pull request into `main`.
4. Complete the impact and verification sections in the pull-request template.
5. Obtain at least one approval and satisfy every required status check.
6. Squash merge after review; do not merge directly to `main`.

The pull-request title becomes the final squash commit subject. Intermediate
branch commits do not need to follow Conventional Commits.

## Pull-request title convention

```text
<type>(<optional-scope>)<optional-breaking-marker>: <description>
```

Allowed types are `feat`, `fix`, `docs`, `refactor`, `test`, `build`,
`ci`, `chore`, `perf`, and `revert`. An optional scope uses lowercase
letters, digits, and hyphens. The description must be non-empty.

Validate locally:

```bash
go run ./scripts/validate-pr-title -title 'feat(order): add checkout validation'
```

Valid examples:

- `feat(order): add checkout validation`
- `fix!: reject reused refresh tokens`
- `docs: clarify local startup`

Invalid examples:

- `Feat(order): add checkout validation` (uppercase type)
- `feat(order_api): add checkout validation` (underscore in scope)
- `feat(order) add checkout validation` (missing colon-space)
- `feat(order):` (empty description)

## CI policy

CI runs on pull requests targeting `main` and on pushes to `main`. The required
checks are independent so branch protection can identify the failing surface.

| Check | Responsibility |
| --- | --- |
| PR title | Validate the pull-request title with `scripts/validate-pr-title`. |
| Boundaries | Run `scripts/check-boundaries` to reject cross-service imports, build/database dependencies, internal Kong (`:8000`) calls, contract-to-service imports, and frontend-to-backend/contract imports. |
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

## Branch protection settings

Tracked documentation records the intended settings; it does not configure
GitHub remotely. Configure the `main` branch to:

- require a pull request before merging;
- require one approval and dismiss stale approvals;
- require conversation resolution;
- require the checks `PR title`, `Boundaries`, `Contracts`, `Java`, `Go`, and `Frontend`;
- require the branch to be up to date if the repository team adopts that setting consistently;
- block force pushes and branch deletion;
- enable squash merge and disable merge commits and rebase merges.
