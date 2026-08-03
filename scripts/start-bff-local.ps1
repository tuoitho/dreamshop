[CmdletBinding()]
param()

$repoRoot = Split-Path -Parent $PSScriptRoot
$env:ENV_FILE = Join-Path $repoRoot ".env"

if (-not (Test-Path -LiteralPath $env:ENV_FILE)) {
    Write-Warning "Local env file was not found at $env:ENV_FILE. Defaults will be used."
}

Push-Location $repoRoot
try {
    & go run ".\services\bff-service\cmd\server"
}
finally {
    Pop-Location
}
