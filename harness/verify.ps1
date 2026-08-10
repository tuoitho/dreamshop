#Requires -Version 7
[CmdletBinding()]
param(
    [ValidateSet('discovery', 'contract', 'surface', 'integration', 'e2e', 'all')]
    [string]$Stage = 'all',
    [string[]]$Surface,
    [string]$ManifestPath,
    [switch]$DryRun
)

$engine = Join-Path (Split-Path $PSScriptRoot -Parent) '.agents/skills/fullstack-feature/scripts/verify.ps1'
& $engine -Stage $Stage -Surface $Surface -RepoRoot (Split-Path $PSScriptRoot -Parent) -ManifestPath $ManifestPath -DryRun:$DryRun
exit $LASTEXITCODE
