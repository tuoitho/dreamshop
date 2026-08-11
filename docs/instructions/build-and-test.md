# Build and Test

Choose the smallest scope that gives sufficient confidence, following the
[testing steering rules](../steering/testing.md). There is no universal wrapper
that replaces selecting the relevant direct checks.

## Focused checks

```powershell
mvn -pl services/identity-service -am test
mvn -pl services/order-service -am test
go test ./services/bff-service/internal/...
go test ./services/catalog-service/internal/...
go test ./services/inventory-service/internal/...
npm --prefix ecommerce-fe test -- --run src/lib/api.test.ts
```

## Surface checks

```powershell
mvn -pl services/identity-service -am verify
mvn -pl services/order-service -am verify
go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
go build ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
npm --prefix ecommerce-fe test -- --run
npm --prefix ecommerce-fe run lint
npm --prefix ecommerce-fe run build
```

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
go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
go build ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
npm --prefix ecommerce-fe ci
npm --prefix ecommerce-fe test -- --run
npm --prefix ecommerce-fe run lint
npm --prefix ecommerce-fe run build
```

The existing `make verify`, `make verify-identity`, and
`make verify-edge-auth` scripts are focused foundation/runtime checks. They may
require local infrastructure and running applications; they are not a
replacement for the direct build and test commands above.
