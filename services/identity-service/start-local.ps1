$ErrorActionPreference = "Stop"

$serviceDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent (Split-Path -Parent $serviceDir)

$env:IDENTITY_JWT_PRIVATE_KEY_PATH = Join-Path $repoRoot "deployments\secrets\identity\private.pem"
$env:IDENTITY_JWT_PUBLIC_KEY_PATH = Join-Path $repoRoot "deployments\secrets\identity\public.pem"
$env:SERVER_PORT = "8080"
$env:IDENTITY_JWT_ISSUER = "http://localhost:8080"
$env:IDENTITY_SESSION_COOKIE_SECURE = "false"

Push-Location $repoRoot
try {
    mvn -f services/identity-service/pom.xml spring-boot:run
} finally {
    Pop-Location
}
