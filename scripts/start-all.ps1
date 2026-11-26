# Starts every backend service in dependency order and waits for each health check.
# Services that already answer their health check are skipped.
#
#   .\scripts\start-all.ps1        start all services (logs in tmp\logs\)
#   .\scripts\start-all.ps1 -Stop  stop the processes this script started
param([switch]$Stop)

$ErrorActionPreference = 'Stop'
$root = Split-Path $PSScriptRoot -Parent
$logDir = Join-Path $root 'tmp\logs'
$pidFile = Join-Path $logDir 'pids.txt'

# Identity first: Catalog, Inventory, Cart, and Order call its authorization gRPC.
# BFF last: it calls every other service.
$services = @(
    @{ Name = 'identity-service';  Health = 'http://localhost:8090/actuator/health' }
    @{ Name = 'catalog-service';   Health = 'http://localhost:8085/healthz' }
    @{ Name = 'inventory-service'; Health = 'http://localhost:8082/healthz' }
    @{ Name = 'cart-service';      Health = 'http://localhost:8084/healthz' }
    @{ Name = 'payment-service';   Health = 'http://localhost:8086/healthz' }
    @{ Name = 'order-service';     Health = 'http://localhost:8093/actuator/health' }
    @{ Name = 'bff-service';       Health = 'http://localhost:8081/healthz' }
)

if ($Stop) {
    if (Test-Path $pidFile) {
        # /T kills the whole tree: `go run` and `mvn` each spawn the real server as a child.
        Get-Content $pidFile | ForEach-Object { taskkill /T /F /PID $_ 2>&1 | Out-Null }
        Remove-Item $pidFile
    }
    Write-Host 'Stopped.'
    return
}

function Test-Health([string]$Url) {
    try { Invoke-WebRequest $Url -UseBasicParsing -TimeoutSec 2 | Out-Null; return $true } catch { return $false }
}

New-Item -ItemType Directory -Force $logDir | Out-Null

foreach ($svc in $services) {
    if (Test-Health $svc.Health) { Write-Host "$($svc.Name): already UP"; continue }

    $log = Join-Path $logDir "$($svc.Name).log"
    $proc = Start-Process go -ArgumentList 'run', './scripts/start-local', '-service', $svc.Name `
        -WorkingDirectory $root -WindowStyle Hidden -PassThru `
        -RedirectStandardOutput $log -RedirectStandardError "$log.err"
    Add-Content $pidFile $proc.Id
    Write-Host -NoNewline "$($svc.Name): starting (PID $($proc.Id))"

    $deadline = (Get-Date).AddMinutes(3)
    while (-not (Test-Health $svc.Health)) {
        if ($proc.HasExited -or (Get-Date) -gt $deadline) {
            Write-Host " FAILED - see $log and $log.err"
            exit 1
        }
        Start-Sleep -Seconds 2
        Write-Host -NoNewline '.'
    }
    Write-Host ' UP'
}

Write-Host 'All services UP. Stop with: .\scripts\start-all.ps1 -Stop'
