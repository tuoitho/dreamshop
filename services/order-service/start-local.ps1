$ErrorActionPreference = "Stop"

$serviceDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent (Split-Path -Parent $serviceDir)

Push-Location $repoRoot
try {
    mvn -f services/order-service/pom.xml spring-boot:run
} finally {
    Pop-Location
}
