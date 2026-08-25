[CmdletBinding()]
param(
    [string]$BootstrapSuperAdminEmail,
    [string]$BootstrapSuperAdminFullName
)

$ErrorActionPreference = "Stop"

$serviceDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent (Split-Path -Parent $serviceDir)

$env:IDENTITY_JWT_PRIVATE_KEY_PATH = Join-Path $repoRoot "deployments\secrets\identity\private.pem"
$env:IDENTITY_JWT_PUBLIC_KEY_PATH = Join-Path $repoRoot "deployments\secrets\identity\public.pem"
$env:SERVER_PORT = "8080"
$env:IDENTITY_JWT_ISSUER = "http://localhost:8080"
$env:IDENTITY_SESSION_COOKIE_SECURE = "false"
if (-not $env:IDENTITY_GRPC_HOST) { $env:IDENTITY_GRPC_HOST = "127.0.0.1" }
if (-not $env:IDENTITY_GRPC_PORT) { $env:IDENTITY_GRPC_PORT = "50054" }
if (-not $env:IDENTITY_GRPC_SHUTDOWN_GRACE) { $env:IDENTITY_GRPC_SHUTDOWN_GRACE = "5s" }

$previousBootstrapEmail = $env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL
$previousBootstrapPassword = $env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD
$previousBootstrapFullName = $env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME

$bootstrapEmail = if ($PSBoundParameters.ContainsKey('BootstrapSuperAdminEmail')) {
    $BootstrapSuperAdminEmail.Trim()
} else {
    $env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL
}
$bootstrapFullName = if ($PSBoundParameters.ContainsKey('BootstrapSuperAdminFullName')) {
    $BootstrapSuperAdminFullName.Trim()
} else {
    $env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME
}
$bootstrapPassword = $env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD
$bootstrapRequested = -not [string]::IsNullOrWhiteSpace($bootstrapEmail) `
    -or -not [string]::IsNullOrWhiteSpace($bootstrapPassword) `
    -or -not [string]::IsNullOrWhiteSpace($bootstrapFullName)

if ($bootstrapRequested) {
    if ([string]::IsNullOrWhiteSpace($bootstrapEmail) -or [string]::IsNullOrWhiteSpace($bootstrapFullName)) {
        throw 'Bootstrap SUPER_ADMIN email and full name must be supplied together.'
    }
    if ([string]::IsNullOrWhiteSpace($bootstrapPassword)) {
        $securePassword = Read-Host 'Bootstrap SUPER_ADMIN password' -AsSecureString
        if ($securePassword.Length -lt 12 -or $securePassword.Length -gt 128) {
            throw 'Bootstrap SUPER_ADMIN password must be 12 through 128 characters.'
        }
        $passwordPointer = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($securePassword)
        try {
            $bootstrapPassword = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($passwordPointer)
        } finally {
            [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($passwordPointer)
        }
    }

    $env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL = $bootstrapEmail
    $env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD = $bootstrapPassword
    $env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME = $bootstrapFullName
    $bootstrapPassword = $null
}

Push-Location $repoRoot
try {
    mvn -f services/identity-service/pom.xml spring-boot:run
} finally {
    Pop-Location
    $env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_EMAIL = $previousBootstrapEmail
    $env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_PASSWORD = $previousBootstrapPassword
    $env:IDENTITY_BOOTSTRAP_SUPER_ADMIN_FULL_NAME = $previousBootstrapFullName
}
