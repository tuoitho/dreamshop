# BFF Local Environment Configuration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `bff-service` load local `.env` values through `godotenv` and Viper while remaining compatible with environment injection from Kubernetes.

**Architecture:** The service loads an optional local `.env` file before constructing its Viper configuration. Viper reads environment variables with the `BFF_` prefix and applies defaults; existing generic `HTTP_ADDR` support remains compatible with the current Compose setup. The application does not depend on `.env` in production.

**Tech Stack:** Go 1.24, Viper, godotenv, PowerShell, Go tests.

---

### Task 1: Define the BFF configuration contract with tests

**Files:**
- Modify: `services/bff-service/internal/config/config_test.go`
- Modify: `services/bff-service/internal/config/config.go`

- [ ] **Step 1: Replace the required-variable test with default and environment tests**

Cover these cases:

```go
func TestLoadUsesDefaults(t *testing.T) {
    t.Setenv("BFF_HTTP_ADDR", "")
    t.Setenv("HTTP_ADDR", "")
    t.Setenv("BFF_CATALOG_GRPC_ADDR", "")

    cfg, err := Load()
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if cfg.HTTPAddr != ":8080" {
        t.Fatalf("expected default HTTP address, got %q", cfg.HTTPAddr)
    }
    if cfg.CatalogGRPCAddr != "127.0.0.1:50051" {
        t.Fatalf("expected default Catalog gRPC address, got %q", cfg.CatalogGRPCAddr)
    }
}
```

Also verify `BFF_HTTP_ADDR` and `BFF_CATALOG_GRPC_ADDR` override defaults, and verify legacy `HTTP_ADDR` remains supported when the prefixed variable is empty.

- [ ] **Step 2: Run the focused test and confirm it fails before implementation**

Run from `E:\microservice`:

```powershell
go test ./services/bff-service/internal/config
```

Expected failure: `Config` does not yet contain `CatalogGRPCAddr` and `Load` still requires `HTTP_ADDR`.

### Task 2: Implement optional dotenv loading and Viper binding

**Files:**
- Modify: `services/bff-service/internal/config/config.go`
- Modify: `services/bff-service/go.mod`
- Create or modify: `services/bff-service/go.sum`

- [ ] **Step 1: Add `godotenv` and Viper dependencies**

Add `github.com/joho/godotenv` and `github.com/spf13/viper` to the BFF module.

- [ ] **Step 2: Load `.env` without overriding injected environment variables**

Use `godotenv.Load`, never `godotenv.Overload`. Treat a missing `.env` as normal so K8s and CI can run without the file; return unexpected file errors.

- [ ] **Step 3: Bind the configuration**

Expose:

```go
type Config struct {
    HTTPAddr        string
    CatalogGRPCAddr string
}
```

Use these environment variables and defaults:

```text
BFF_HTTP_ADDR          -> :8080
HTTP_ADDR              -> :8080 (legacy fallback)
BFF_CATALOG_GRPC_ADDR  -> 127.0.0.1:50051
```

Use `SetEnvKeyReplacer` and explicit `BindEnv` calls so Viper values also unmarshal reliably.

- [ ] **Step 4: Run the focused test and confirm it passes**

```powershell
go test ./services/bff-service/internal/config
```

Expected output: `ok ecommerce.local/bff-service/internal/config`.

### Task 3: Wire the BFF process to the new config

**Files:**
- Modify: `services/bff-service/cmd/server/main.go`
- Modify: `services/bff-service/internal/config/config_test.go`

- [ ] **Step 1: Remove the manual `HTTP_ADDR` mutation from `main.go`**

Let `config.Load()` own defaults and environment handling. Keep the existing health endpoint and log both configured addresses.

- [ ] **Step 2: Run all BFF tests**

```powershell
go test ./services/bff-service/...
```

Expected output: all BFF packages pass.

### Task 4: Document local values without committing secrets

**Files:**
- Modify: `.env.example`
- Create: `scripts/start-bff-local.ps1`

- [ ] **Step 1: Add BFF examples to `.env.example`**

Add:

```dotenv
BFF_HTTP_ADDR=:8080
BFF_CATALOG_GRPC_ADDR=127.0.0.1:50051
```

- [ ] **Step 2: Add a PowerShell launcher**

The launcher sets `ENV_FILE` to the repository `.env`, starts the BFF from the repository root so dotenv discovery is deterministic, and runs:

```powershell
go run .\services\bff-service\cmd\server
```

It must not print secret values or create a `.env` file.

- [ ] **Step 3: Test the launcher and health endpoint**

Start it in one terminal, then from another terminal run:

```powershell
Invoke-WebRequest http://localhost:8080/healthz
```

Expected HTTP status: `200`.

### Task 5: Verify module formatting and dependency consistency

- [ ] **Step 1: Format changed Go files**

```powershell
gofmt -w .\services\bff-service\internal\config\config.go .\services\bff-service\internal\config\config_test.go .\services\bff-service\cmd\server\main.go
```

- [ ] **Step 2: Run BFF tests and inspect the diff**

```powershell
go test ./services/bff-service/...
git diff -- services/bff-service .env.example scripts/start-bff-local.ps1
```

Expected result: tests pass and only BFF configuration/launcher files are changed, in addition to this plan.
