$ErrorActionPreference = "Stop"

$serviceDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent (Split-Path -Parent $serviceDir)

Push-Location $serviceDir
try {
    if (-not $env:DATABASE_URL) { $env:DATABASE_URL = "postgres://ecommerce:change-me-local-only@127.0.0.1:5432/cart_db?sslmode=disable" }
    if (-not $env:HTTP_ADDR) { $env:HTTP_ADDR = ":8084" }
    if (-not $env:GRPC_ADDR) { $env:GRPC_ADDR = "127.0.0.1:50053" }
    if (-not $env:CATALOG_GRPC_ADDR) { $env:CATALOG_GRPC_ADDR = "127.0.0.1:50051" }
    if (-not $env:CATALOG_GRPC_TIMEOUT) { $env:CATALOG_GRPC_TIMEOUT = "3s" }
    if (-not $env:INVENTORY_GRPC_ADDR) { $env:INVENTORY_GRPC_ADDR = "127.0.0.1:50052" }
    if (-not $env:INVENTORY_GRPC_TIMEOUT) { $env:INVENTORY_GRPC_TIMEOUT = "3s" }
    if (-not $env:IDENTITY_GRPC_ADDR) { $env:IDENTITY_GRPC_ADDR = "127.0.0.1:50054" }
    if (-not $env:IDENTITY_GRPC_TIMEOUT) { $env:IDENTITY_GRPC_TIMEOUT = "500ms" }

    go run ./cmd/server
} finally {
    Pop-Location
}
