$ErrorActionPreference = "Stop"

$serviceDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent (Split-Path -Parent $serviceDir)

$env:IDENTITY_JWT_PRIVATE_KEY_PATH = Join-Path $repoRoot "deployments\secrets\identity\private.pem"
$env:IDENTITY_JWT_PUBLIC_KEY_PATH = Join-Path $repoRoot "deployments\secrets\identity\public.pem"

Push-Location $repoRoot
try {
    mvn -f services/identity-service/pom.xml spring-boot:run
} finally {
    Pop-Location
}
