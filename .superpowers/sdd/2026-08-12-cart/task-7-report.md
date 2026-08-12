# Task 7 Report

## Status

DONE_WITH_CONCERNS

## Target and ownership

Implemented Task 7 within the Cart service surface only. The Cart service owns
`cart_db`, its handwritten migration, private persistence adapter, configuration,
and pure cart status rules. No contracts, BFF, frontend, or other service source
was modified.

## Implemented

- Added the Cart Go module and workspace registration, local start scripts, and
  Makefile targets.
- Retained the required environment-backed configuration and tests for required
  `DATABASE_URL`, Cart/default addresses, dependency addresses, and positive
  dependency timeouts.
- Retained pure `domain.ResolveStatus` and its table-driven coverage.
- Added the handwritten Cart migration for `carts` and `cart_items`.
- Added sqlc v2 configuration at `services/cart-service/sqlc.yaml`.
- Added service-private SQL query definitions for `EnsureCart`, `FindCart`,
  `ListItems`, `AddItem`, `UpdateQuantity`, `RemoveItem`, and `Clear`.
- Added checked-in generated Go code under
  `services/cart-service/internal/store/db`.
- Rewrote `store.go` as a thin adapter over generated sqlc methods. It maps
  `pgx.ErrNoRows` to `FindCart`'s `(found=false)` result, maps zero affected
  rows from update/remove to `ErrItemNotFound`, and translates generated rows
  into `domain.Line` without embedding CRUD SQL.
- Integration coverage verifies same-user cart identity, duplicate-line quantity
  upsert, cart-scoped update/remove protection, clear behavior, missing-cart
  lookup, and missing-item sentinel mapping. Integration tests skip when
  `DATABASE_URL` is absent.

## Generation evidence

Generated with the reproducible command below from
`services/cart-service`:

```bash
GOTOOLCHAIN=local go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate -f sqlc.yaml
```

The command reported `sqlc v1.27.0`. `sqlc vet` was also run with the same
pinned module/version and passed. Generated files report `sqlc v1.27.0` in their
headers. The migration remains handwritten as required.

## Verification

Passed:

```text
GOTOOLCHAIN=local go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 version
GOTOOLCHAIN=local go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 vet -f sqlc.yaml
GOTOOLCHAIN=local go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0 generate -f sqlc.yaml
git diff --check
```

Using the pulled `golang:1.24` Docker image (`go1.24.13`), the focused suite
passed against the live local PostgreSQL instance:

```text
docker run --rm --network host \
  -e DATABASE_URL='postgres://ecommerce:change-me-local-only@127.0.0.1:5432/cart_db?sslmode=disable' \
  -v /home/tt/dreamshop/.worktrees/cart-feature:/src -w /src \
  --entrypoint /usr/local/go/bin/go golang:1.24 \
  test ./services/cart-service/... -count=1
```

Result: config, domain, store, and generated private store package compiled;
all Cart tests passed. The missing local `cart_db` database was created in the
existing PostgreSQL container before the integration rerun; no repository
deployment files were changed.

The requested host command was also attempted:

```text
GOTOOLCHAIN=local go test ./services/cart-service/internal/domain ./services/cart-service/internal/store -count=1
```

It is blocked before compilation because the host has Go 1.22.2 while the
repository requires Go 1.24. The Docker Go 1.24 run is the equivalent successful
verification. `go -C services/cart-service mod tidy` was run in Go 1.24 Docker;
because this Task 7 module intentionally predeclares the future contracts/gRPC
dependencies required by the brief while no `cmd/server` exists yet, tidy would
remove those currently-unused direct requirements. The required module file was
therefore retained rather than accepting a premature dependency-shape change.

## Concerns and residual risks

- The host Go toolchain remains Go 1.22.2, so native host Cart tests cannot run
  until Go 1.24 is installed. Docker-based Go 1.24 verification passed.
- The existing PostgreSQL volume predated Cart and did not contain `cart_db`, so
  it required one-time local database creation. A fresh foundation initialized
  from the repository's existing PostgreSQL init script should create it.
- The service has no `cmd/server` in Task 7's ownership list; the local scripts
  intentionally point to the prescribed future entrypoint and are not executable
  until the server task lands.
- `go work sync` was not retained as a broad workspace mutation: under the
  current snapshot it rewrote unrelated module files and dependency sums. Those
  unrelated changes were reverted; the Cart workspace registration remains.

## Commit scope

Only Task 7-owned repository files, the service-private sqlc inputs/generated
output, and this report are intended for the Task 7 commit.
