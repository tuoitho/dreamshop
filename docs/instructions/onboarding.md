# Onboarding

## Supported prerequisites

- Java 21 and Maven 3.9 or newer.
- Go 1.24 or newer within the 1.24 line used by the workspace.
- Node.js 22.12 or newer for Vite 8, with npm from that Node release.
- Docker Desktop with Docker Compose v2.
- Buf CLI 1.70 or newer.
- PowerShell 7 (`pwsh`).
- GNU Make when using Makefile shortcuts.

Check the installed tools from PowerShell:

```powershell
java -version
mvn -version
go version
node --version
npm --version
docker compose version
buf --version
pwsh --version
make --version
```

## First checkout

From the repository root:

```powershell
git clone <repository-url>
Set-Location microservice
Copy-Item .env.example .env
make identity-keys
npm --prefix ecommerce-fe ci
buf generate
```

The `.env` file and generated PEM files are local-only. Never commit them or
reuse development keys outside local development.

## Narrow smoke checks

Run only the checks needed for the first change:

```powershell
buf lint
go test ./services/bff-service/... ./services/catalog-service/... ./services/inventory-service/...
mvn --batch-mode --no-transfer-progress test
npm --prefix ecommerce-fe test -- --run
```

Read the [service catalog](../steering/service-catalog.md) before starting a
service, and continue with the [local-development guide](local-development.md)
when a runtime dependency is required. Do not start every application for a
domain-only change.
