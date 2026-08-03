$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$keyDir = Join-Path $repoRoot 'deployments/secrets/identity'
$privatePath = Join-Path $keyDir 'private.pem'
$publicPath = Join-Path $keyDir 'public.pem'

if ((Test-Path -LiteralPath $privatePath) -xor (Test-Path -LiteralPath $publicPath)) {
    throw 'Identity key pair is incomplete; remove the remaining file deliberately and rerun.'
}
if ((Test-Path -LiteralPath $privatePath) -and (Test-Path -LiteralPath $publicPath)) {
    Write-Host 'Identity development key pair already exists.'
    exit 0
}

New-Item -ItemType Directory -Force -Path $keyDir | Out-Null
$rsa = [System.Security.Cryptography.RSA]::Create(2048)
try {
    [System.IO.File]::WriteAllText($privatePath, $rsa.ExportPkcs8PrivateKeyPem())
    [System.IO.File]::WriteAllText($publicPath, $rsa.ExportSubjectPublicKeyInfoPem())
} finally {
    $rsa.Dispose()
}
Write-Host 'Created untracked Identity development key pair.'
