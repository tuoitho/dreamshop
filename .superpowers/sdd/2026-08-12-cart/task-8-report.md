# Task 8 Report

## Status

DONE_WITH_CONCERNS

## Target and ownership

Implemented Task 8 within the Cart application surface only. Cart owns the gRPC
application validation, Catalog and Inventory private gRPC adapters, process
startup/lifecycle, and `/healthz`. Catalog and Inventory are consumed only via
generated contracts. No BFF, Kong, frontend, other service source, or Cart SQL
execution semantics were changed.

## Implemented

- Added Cart Catalog and Inventory adapters with lazy insecure gRPC connections,
  per-call deadlines, interface-friendly methods, and `Close()`.
- Added the Cart gRPC server with small store/catalog/inventory interfaces.
- Validated required `user_id`, `product_id`, `variant_label`, `item_id`, and
  positive mutation quantities with `codes.InvalidArgument`.
- Added `FindCart` handling for `GetCart`: absent carts return an empty Cart and
  do not create persistence rows.
- Wired all mutations through `EnsureCart`, the existing sqlc-backed Store
  methods, and the shared refreshed-cart enrichment helper. Mutations return
  their method-specific response wrappers.
- Enriched each line from Catalog and Inventory, including current product
  fields, first image, matching variant color, availability, and prefixed Cart
  status enums.
- Mapped Catalog `NotFound` to an unavailable non-degraded row; deleted products
  and removed variants likewise remain unavailable without an Inventory call.
- Mapped Catalog/Inventory transport failures to unavailable rows with
  `degraded=true`. Catalog fields remain available when only Inventory is down.
- Added bufconn/fake behavior coverage for all four statuses, limited quantity
  preservation, Catalog outage degradation, Catalog not-found behavior,
  Inventory outage enrichment preservation, empty-cart lookup, input validation,
  user forwarding, mutation wrappers, and clear behavior.
- Added Cart `/healthz` and startup wiring for config, PostgreSQL ping/migration,
  generated-contract registration, gRPC/HTTP serving, and SIGINT/SIGTERM
  graceful shutdown. Startup error logs avoid printing the database DSN.

## TDD evidence

Initial focused test run before production implementation:

```text
docker run --rm --network host \
  -v /home/tt/dreamshop/.worktrees/cart-feature:/src -w /src \
  --entrypoint /usr/local/go/bin/go golang:1.24 \
  test ./services/cart-service/internal/grpc -count=1
```

Result: expected RED; compilation failed because the requested `Server` and
`NewServer` production API did not exist yet.

The additional Inventory outage regression was then run before its fix:

```text
docker run --rm --network host \
  -v /home/tt/dreamshop/.worktrees/cart-feature:/src -w /src \
  --entrypoint /usr/local/go/bin/go golang:1.24 \
  test ./services/cart-service/internal/grpc \
  -run TestGetCartInventoryOutagePreservesCatalogEnrichment -count=1
```

Result: expected RED; the row incorrectly lost Catalog enrichment when
Inventory failed. The implementation was changed to map Catalog fields before
the Inventory call, then the focused regression and related enrichment tests
passed.

## Verification

Passed in Go 1.24 Docker (`go1.24` image):

```text
docker run --rm -v /home/tt/dreamshop/.worktrees/cart-feature:/src -w /src \
  bufbuild/buf:1.47.2 format --diff --exit-code

docker run --rm -v /home/tt/dreamshop/.worktrees/cart-feature:/src -w /src \
  bufbuild/buf:1.47.2 lint

docker run --rm -v /home/tt/dreamshop/.worktrees/cart-feature:/src -w /src \
  bufbuild/buf:1.47.2 generate

docker run --rm --network host \
  -e DATABASE_URL='postgres://ecommerce:change-me-local-only@127.0.0.1:5432/cart_db?sslmode=disable' \
  -v /home/tt/dreamshop/.worktrees/cart-feature:/src -w /src \
  --entrypoint /usr/local/go/bin/go golang:1.24 \
  test ./services/cart-service/... -count=1
```

Full Cart result: PASS. Config, domain, gRPC server, adapters, HTTP package,
startup package, sqlc-backed store, migration, and generated private store
package all compiled; Cart tests passed. `buf format`, `buf lint`, and
`buf generate` passed. `git diff --check` passed. Contract generation produced
no Cart proto or generated-source changes because the existing Task 7/earlier
contract output already matched the brief.

## Concerns and residual risks

- The host toolchain is Go 1.22.2 while Cart requires Go 1.24; host-native Cart
  tests were not used. Required Go 1.24 Docker verification passed.
- gRPC auth is represented by the existing contract `user_id` request fields;
  no separate metadata/auth interceptor was specified in the Task 8 brief.
  BFF/Kong caller-context authentication remains explicitly out of scope for
  this task and is owned by later BFF work.
- Startup has no end-to-end process-signal test; lifecycle follows the existing
  Catalog/Inventory process shape and the command package builds successfully.
- The local PostgreSQL-backed store tests rely on the existing `cart_db`
  foundation and skip/fail according to the repository's existing
  `DATABASE_URL` convention.

## Commit scope

Only Task 8-owned Cart transport/process files and this report are intended for
the Task 8 commit. No generated contract files changed.
