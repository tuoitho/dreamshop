# Build and Test

Choose the smallest scope that gives sufficient confidence, following the
[testing steering rules](../steering/testing.md). Focused checks are selected
directly; surface and contract gates also have a single-command wrapper.

## Focused checks

```powershell
mvn -pl services/identity-service -am test
mvn -pl services/order-service -am test
go test ./services/bff-service/internal/...
go test ./services/catalog-service/internal/...
go test ./services/inventory-service/internal/...
go test ./services/cart-service/internal/...
npm --prefix ecommerce-fe test -- --run src/lib/api.test.ts
```

## Surface checks

`make verify SERVICE=<surface>` runs `scripts/check-boundaries` and then the
gate for one surface. Its exit code is the gate result.

| `SERVICE` | Gate |
| --- | --- |
| Java service, e.g. `identity-service` | `mvn -pl services/<name> -am verify` |
| Go service, e.g. `cart-service` | `go build` and `go test` for `./services/<name>/...` |
| `ecommerce-fe` | Frontend tests, lint, and build |
| `contracts` | Contract checks below, plus `buf breaking` against `main` |

On Windows, the `cart-service` gate tests `./services/cart-service/internal/...`
only, for the reason described at the end of this document.

The equivalent direct commands are:

```powershell
mvn -pl services/identity-service -am verify
mvn -pl services/order-service -am verify
go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/...
go build ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/...
npm --prefix ecommerce-fe test -- --run
npm --prefix ecommerce-fe run lint
npm --prefix ecommerce-fe run build
```

## Boundary check

```powershell
make check-boundaries
```

The check fails on Go or Java imports of another service, Maven dependencies on
another service module, references to another service's `<name>_db` database,
internal service calls through Kong (`:8000`), contract modules importing
service code, and frontend imports of backend or contract source. Rules that
need semantic judgment, such as copied domain logic or holding a database
transaction across a network call, are covered by the boundary review in
[`service-boundaries.md`](../steering/service-boundaries.md).

## Contract checks

```powershell
$gitRoot = Split-Path (Split-Path (Get-Command git).Source -Parent) -Parent
$env:PATH = "$gitRoot\usr\bin;$env:PATH"
buf format --diff --exit-code
buf lint
buf generate
git diff --exit-code -- contracts/go contracts/java/src/main/generated
git ls-files --others --exclude-standard -- contracts/go contracts/java/src/main/generated
```

The Git for Windows `usr\bin` path provides the `diff` executable used by
`buf format --diff`; it is unnecessary on Linux CI. Any output from the last
command means a generated file has not been staged. Run the affected language
build after changing generated contracts. Protobuf sources live under
`contracts/`; generated Java and Go modules are consumed by their owning
services.

## Repository checks

The CI-equivalent direct sequence is:

```powershell
buf format --diff --exit-code
buf lint
buf generate
git diff --exit-code -- contracts/go contracts/java/src/main/generated
mvn --batch-mode --no-transfer-progress verify
go test ./scripts/... ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/...
go build ./scripts/... ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/... ./services/cart-service/...
npm --prefix ecommerce-fe ci
npm --prefix ecommerce-fe test -- --run
npm --prefix ecommerce-fe run lint
npm --prefix ecommerce-fe run build
```

Without `SERVICE`, `make verify` runs the foundation/runtime check. It may
require local infrastructure and running applications; it is not a
replacement for the direct build and test commands above.

The current `cart-service/cmd/server` tests use Unix process signaling. On
Windows, `go test ./services/cart-service/...` fails to compile that test
package because `syscall.Kill` is unavailable; the Cart internal-package check
above remains usable. Run the complete Cart test command on Linux/CI until the
server lifecycle test is made portable.
