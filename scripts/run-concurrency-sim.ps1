param(
    [ValidateSet("all", "flash-sale-oversell", "snapshot-mismatch-race", "idempotency-storm", "momo-callback-race")]
    [string]$Scenario = "all",
    [string]$BaseUrl = "http://localhost:8000/api/v1",
    [string]$AdminToken = $env:SIM_ADMIN_TOKEN,
    [string]$CustomerToken = $env:SIM_CUSTOMER_TOKEN,
    [string]$AdminEmail = $env:SIM_ADMIN_EMAIL,
    [string]$AdminPassword = $env:SIM_ADMIN_PASSWORD,
    [string]$CustomerEmail = $env:SIM_CUSTOMER_EMAIL,
    [string]$CustomerPassword = $env:SIM_CUSTOMER_PASSWORD,
    [switch]$Json
)

$ErrorActionPreference = "Stop"
$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $RepoRoot
try {
    $goArgs = @(
        "run", "./scripts/concurrency-sim",
        "-scenario", $Scenario,
        "-base-url", $BaseUrl
    )
    if ($AdminToken) { $goArgs += @("-admin-token", $AdminToken) }
    if ($CustomerToken) { $goArgs += @("-customer-token", $CustomerToken) }
    if ($AdminEmail) { $goArgs += @("-admin-email", $AdminEmail) }
    if ($AdminPassword) { $goArgs += @("-admin-password", $AdminPassword) }
    if ($CustomerEmail) { $goArgs += @("-customer-email", $CustomerEmail) }
    if ($CustomerPassword) { $goArgs += @("-customer-password", $CustomerPassword) }
    if ($Json) { $goArgs += "-json" }

    & go @goArgs
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
