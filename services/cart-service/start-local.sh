#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/../.." && pwd)"

cd "$repo_root/services/cart-service"
: "${DATABASE_URL:=postgres://ecommerce:change-me-local-only@127.0.0.1:5432/cart_db?sslmode=disable}"
: "${HTTP_ADDR:=:8084}"
: "${GRPC_ADDR:=127.0.0.1:50053}"
: "${CATALOG_GRPC_ADDR:=127.0.0.1:50051}"
: "${CATALOG_GRPC_TIMEOUT:=3s}"
: "${INVENTORY_GRPC_ADDR:=127.0.0.1:50052}"
: "${INVENTORY_GRPC_TIMEOUT:=3s}"
: "${IDENTITY_GRPC_ADDR:=127.0.0.1:50054}"
: "${IDENTITY_GRPC_TIMEOUT:=500ms}"
export DATABASE_URL HTTP_ADDR GRPC_ADDR CATALOG_GRPC_ADDR CATALOG_GRPC_TIMEOUT INVENTORY_GRPC_ADDR INVENTORY_GRPC_TIMEOUT IDENTITY_GRPC_ADDR IDENTITY_GRPC_TIMEOUT

go run ./cmd/server
