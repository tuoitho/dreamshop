param(
    [ValidateSet("all", "flash-sale-oversell", "snapshot-mismatch-race", "idempotency-storm", "momo-callback-race")]
    [string]$Scenario = "all",
    [string]$BaseUrl = "http://localhost:8000/api/v1",
    [string]$AdminToken = $env:SIM_ADMIN_TOKEN,
    [string]$CustomerToken = $env:SIM_CUSTOMER_TOKEN,
    [string]$AdminEmail = $(if ($env:SIM_ADMIN_EMAIL) { $env:SIM_ADMIN_EMAIL } else { "admin@example.com" }),
    [string]$AdminPassword = $(if ($env:SIM_ADMIN_PASSWORD) { $env:SIM_ADMIN_PASSWORD } else { "AdminPassword123!" }),
    [string]$CustomerEmail = $(if ($env:SIM_CUSTOMER_EMAIL) { $env:SIM_CUSTOMER_EMAIL } else { "customer1@gmail.com" }),
    [string]$CustomerPassword = $(if ($env:SIM_CUSTOMER_PASSWORD) { $env:SIM_CUSTOMER_PASSWORD } else { "123456123456" }),
    [string]$SettleTimeout = "",
    [switch]$VerifySql,
    [switch]$Json
)

$ErrorActionPreference = "Stop"
$RepoRoot = Resolve-Path (Join-Path $PSScriptRoot "..")
Push-Location $RepoRoot
try {
    if ([string]::IsNullOrWhiteSpace($CustomerToken) -and -not [string]::IsNullOrWhiteSpace($CustomerEmail) -and -not [string]::IsNullOrWhiteSpace($CustomerPassword)) {
        $registerBody = @{
            email    = $CustomerEmail
            password = $CustomerPassword
            fullName = "Customer One"
        } | ConvertTo-Json -Compress
        try {
            Invoke-RestMethod -Method Post -Uri ($BaseUrl.TrimEnd("/") + "/auth/register") -ContentType "application/json" -Body $registerBody -TimeoutSec 5 | Out-Null
        } catch {
            # Ignore 409 Conflict when the customer account already exists.
        }
    }

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
    if ($SettleTimeout) { $goArgs += @("-settle-timeout", $SettleTimeout) }
    if ($VerifySql) { $goArgs += "-verify-sql" }
    if ($Json) { $goArgs += "-json" }

    & go @goArgs
    exit $LASTEXITCODE
} finally {
    Pop-Location
}
