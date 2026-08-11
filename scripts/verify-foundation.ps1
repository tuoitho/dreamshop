$ErrorActionPreference = 'Stop'

# Ensure go/bin is in PATH for buf
$goBin = "$env:USERPROFILE\go\bin"
if (Test-Path $goBin) {
    if ($env:PATH -notlike "*$goBin*") {
        $env:PATH = "$goBin;$env:PATH"
    }
}

$required = @(
  'buf.yaml', 'buf.gen.yaml', 'deployments/docker/compose.foundation.yml',
  'deployments/kong/kong.yaml', 'contracts/proto/common/v1/common.proto',
  'services/identity-service/pom.xml', 'services/order-service/pom.xml',
  'services/bff-service/go.mod', 'services/catalog-service/go.mod',
  'services/inventory-service/go.mod'
)

$missing = $required | Where-Object { -not (Test-Path $_) }
if ($missing) { throw "Missing foundation files: $($missing -join ', ')" }

docker compose -f deployments/docker/compose.foundation.yml config --quiet
buf lint

# If containers are running, check route health
$containers = docker compose -f deployments/docker/compose.foundation.yml ps -q
if ($containers) {
    Write-Host 'Checking public routes through Kong gateway...'
    try {
        $bffResp = Invoke-WebRequest -Uri 'http://localhost:8000/api/v1/storefront/healthz' -UseBasicParsing -TimeoutSec 5
        if ($bffResp.StatusCode -ne 200) { throw "BFF health route returned status $($bffResp.StatusCode)" }
    } catch {
        throw "Failed to reach BFF through Kong: $_"
    }

    try {
        $identityResp = Invoke-WebRequest -Uri 'http://localhost:8000/api/v1/auth/actuator/health' -UseBasicParsing -TimeoutSec 5
        if ($identityResp.StatusCode -ne 200) { throw "Identity health route returned status $($identityResp.StatusCode)" }
    } catch {
        throw "Failed to reach Identity through Kong: $_"
    }
}

# Documentation assertions
if (Test-Path 'README.md') {
    $readmeContent = Get-Content 'README.md' -Raw
    $requiredTerms = @(
        'make infra-up', 'make proto', 'make verify',
        'bff-service', 'catalog-service', 'inventory-service', 'identity-service', 'order-service',
        '/api/v1/auth', '/api/v1/users', '/api/v1/storefront'
    )
    foreach ($term in $requiredTerms) {
        if ($readmeContent -notlike "*$term*") {
            throw "README.md is missing required instruction or term: $term"
        }
    }
} else {
    throw "README.md does not exist."
}

Write-Host 'Foundation checks passed.'
