#Requires -Version 7
[CmdletBinding()]
param(
    [Parameter()][string]$RepoRoot = (Get-Location).Path,
    [Parameter()][string]$ManifestPath,
    [Parameter()][string]$OutputPath
)

$ErrorActionPreference = 'Stop'

$excludedDirectoryNames = @(
    '.git', '.hg', '.svn', '.mvn', '.gradle', '.m2', 'node_modules', 'vendor',
    'bin', 'obj', 'build', 'dist', 'out', 'target', 'coverage', '.next',
    '.turbo', '.cache'
)
$instructionFileNames = @('AGENTS.md', 'CLAUDE.md', 'GEMINI.md', 'copilot-instructions.md')
$nodeLockFiles = @('package-lock.json', 'npm-shrinkwrap.json', 'pnpm-lock.yaml', 'yarn.lock', 'bun.lockb')
$knownMarkers = @('maven', 'gradle', 'go', 'node')

function ConvertTo-NormalizedPath {
    param([Parameter(Mandatory)][string]$Path)

    return $Path.Replace('\', '/')
}

function Get-RelativePath {
    param(
        [Parameter(Mandatory)][string]$BasePath,
        [Parameter(Mandatory)][string]$Path
    )

    $relativePath = [System.IO.Path]::GetRelativePath($BasePath, $Path)
    if ($relativePath -eq '.') {
        return '.'
    }

    return ConvertTo-NormalizedPath $relativePath
}

function Test-ExcludedRelativePath {
    param([Parameter(Mandatory)][string]$RelativePath)

    $segments = (ConvertTo-NormalizedPath $RelativePath).Split('/', [System.StringSplitOptions]::RemoveEmptyEntries)
    if ($segments -contains 'harness' -and $segments -contains 'reports') {
        for ($index = 0; $index -lt ($segments.Length - 1); $index++) {
            if ($segments[$index] -eq 'harness' -and $segments[$index + 1] -eq 'reports') {
                return $true
            }
        }
    }

    return (@($segments | Where-Object { $_ -in $excludedDirectoryNames }).Count -gt 0)
}

function New-CommandCandidate {
    param(
        [Parameter(Mandatory)][string]$Executable,
        [Parameter(Mandatory)][string[]]$Arguments
    )

    return [PSCustomObject]@{
        executable = $Executable
        arguments = @($Arguments)
    }
}

function Get-ManifestEnvironment {
    param([object]$Environment)

    $normalizedEnvironment = [ordered]@{}
    $issues = @()
    foreach ($property in @($Environment.PSObject.Properties)) {
        if ($property.Name -in @('role', 'ownership', 'dependsOn') -or $property.Value -isnot [string]) {
            $issues += 'manifest environment must be a string-to-string map without semantic fields'
            continue
        }
        $normalizedEnvironment[$property.Name] = [string]$property.Value
    }

    return [PSCustomObject]@{
        environment = [PSCustomObject]$normalizedEnvironment
        issues = @($issues | Sort-Object -Unique)
    }
}

function Get-ManifestCommandCandidates {
    param([object]$Surface)

    $sourceCommands = @()
    if ($null -ne $Surface.commandCandidates) {
        $sourceCommands += @($Surface.commandCandidates)
    }

    if ($null -ne $Surface.checks) {
        foreach ($check in @($Surface.checks.PSObject.Properties)) {
            $sourceCommands += $check.Value
        }
    }

    $commands = @()
    $issues = @()
    foreach ($command in $sourceCommands) {
        if ([string]::IsNullOrWhiteSpace([string]$command.executable) -or $command.arguments -isnot [array]) {
            $issues += 'manifest command must provide executable and argument array'
            continue
        }
        if ($null -ne $command.workingDirectory -and $command.workingDirectory -isnot [string]) {
            $issues += 'manifest workingDirectory must be a string'
            continue
        }
        if ($null -ne $command.timeoutSec -and $command.timeoutSec -isnot [int] -and $command.timeoutSec -isnot [long] -and $command.timeoutSec -isnot [double]) {
            $issues += 'manifest timeoutSec must be numeric'
            continue
        }

        $resolvedCommand = [ordered]@{
            executable = [string]$command.executable
            arguments = @([string[]]$command.arguments)
        }
        foreach ($propertyName in @('workingDirectory', 'timeoutSec')) {
            if ($null -ne $command.$propertyName) {
                $resolvedCommand[$propertyName] = $command.$propertyName
            }
        }
        if ($null -ne $command.environment) {
            $environmentResult = Get-ManifestEnvironment $command.environment
            if ($environmentResult.issues.Count -gt 0) {
                $issues += @($environmentResult.issues)
                continue
            }
            $resolvedCommand.environment = $environmentResult.environment
        }
        $commands += [PSCustomObject]$resolvedCommand
    }

    return [PSCustomObject]@{
        commands = @($commands)
        issues = @($issues)
    }
}

$resolvedRoot = (Resolve-Path -LiteralPath $RepoRoot -ErrorAction Stop).Path
if (-not (Test-Path -LiteralPath $resolvedRoot -PathType Container)) {
    throw "RepoRoot must be a directory: $RepoRoot"
}

$files = @(
    Get-ChildItem -LiteralPath $resolvedRoot -File -Recurse -Force |
        Where-Object { -not (Test-ExcludedRelativePath (Get-RelativePath $resolvedRoot $_.FullName)) }
)

$instructionFiles = @(
    $files |
        Where-Object {
            $_.Name -in $instructionFileNames -or
            (($_.Name -eq 'instructions.md') -and ((ConvertTo-NormalizedPath (Get-RelativePath $resolvedRoot $_.FullName)) -match '(^|/)\.github/'))
        } |
        ForEach-Object { Get-RelativePath $resolvedRoot $_.FullName } |
        Sort-Object -Unique
)

$contracts = @(
    $files |
        Where-Object {
            $relativePath = Get-RelativePath $resolvedRoot $_.FullName
            $normalizedPath = ConvertTo-NormalizedPath $relativePath
            $_.Extension -in @('.proto', '.avsc', '.graphql', '.gql', '.jsonschema') -or
            $normalizedPath -match '(^|/)(openapi|swagger|asyncapi)(/|$)' -or
            $_.Name -match '^(openapi|swagger|asyncapi)(\.[^.]+)?\.(json|ya?ml)$'
        } |
        ForEach-Object { Get-RelativePath $resolvedRoot $_.FullName } |
        Sort-Object -Unique
)

$candidateData = @{}
foreach ($file in $files) {
    $marker = switch -Regex ($file.Name) {
        '^pom\.xml$' { 'maven'; break }
        '^(build|settings)\.gradle(\.kts)?$' { 'gradle'; break }
        '^go\.mod$' { 'go'; break }
        '^package\.json$' { 'node'; break }
        default { $null }
    }

    if ($null -eq $marker) {
        continue
    }

    $candidatePath = Get-RelativePath $resolvedRoot $file.DirectoryName
    if (-not $candidateData.ContainsKey($candidatePath)) {
        $candidateData[$candidatePath] = [ordered]@{
            fullPath = $file.DirectoryName
            markers = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
        }
    }
    [void]$candidateData[$candidatePath].markers.Add($marker)
}

$manifest = $null
$manifestIssues = @()
if ([string]::IsNullOrWhiteSpace($ManifestPath)) {
    $defaultManifestPath = Join-Path $resolvedRoot 'harness/project.json'
    if (Test-Path -LiteralPath $defaultManifestPath -PathType Leaf) {
        $ManifestPath = $defaultManifestPath
    }
}

if (-not [string]::IsNullOrWhiteSpace($ManifestPath)) {
    try {
        $resolvedManifestPath = (Resolve-Path -LiteralPath $ManifestPath -ErrorAction Stop).Path
        $manifest = Get-Content -LiteralPath $resolvedManifestPath -Raw | ConvertFrom-Json -Depth 64
    }
    catch {
        $manifestIssues += [PSCustomObject]@{
            path = ConvertTo-NormalizedPath $ManifestPath
            evidence = @('manifest could not be parsed')
        }
    }
}

if ($null -ne $manifest -and $null -ne $manifest.surfaces) {
    foreach ($surface in @($manifest.surfaces)) {
        if ([string]::IsNullOrWhiteSpace([string]$surface.path)) {
            $manifestIssues += [PSCustomObject]@{
                path = 'manifest'
                evidence = @('manifest surface is missing path')
            }
            continue
        }

        $surfaceFullPath = [System.IO.Path]::GetFullPath((Join-Path $resolvedRoot $surface.path))
        $surfacePath = Get-RelativePath $resolvedRoot $surfaceFullPath
        $rootPrefix = $resolvedRoot.TrimEnd('\', '/') + [System.IO.Path]::DirectorySeparatorChar
        if (($surfaceFullPath -ne $resolvedRoot -and -not $surfaceFullPath.StartsWith($rootPrefix, [System.StringComparison]::OrdinalIgnoreCase)) -or
            -not (Test-Path -LiteralPath $surfaceFullPath -PathType Container)) {
            $manifestIssues += [PSCustomObject]@{
                path = [string]$surface.path
                evidence = @('manifest surface path must be an existing directory within RepoRoot')
            }
            continue
        }

        if (-not $candidateData.ContainsKey($surfacePath)) {
            $candidateData[$surfacePath] = [ordered]@{
                fullPath = $surfaceFullPath
                markers = [System.Collections.Generic.HashSet[string]]::new([System.StringComparer]::OrdinalIgnoreCase)
            }
        }

        $candidateData[$surfacePath].manifestSurface = $surface
        if ($null -ne $surface.markers) {
            $candidateData[$surfacePath].markers.Clear()
            foreach ($marker in @($surface.markers)) {
                if ([string]$marker -in $knownMarkers) {
                    [void]$candidateData[$surfacePath].markers.Add([string]$marker)
                }
                else {
                    $manifestIssues += [PSCustomObject]@{
                        path = $surfacePath
                        evidence = @("unsupported manifest marker: $marker")
                    }
                }
            }
        }
    }
}

$candidates = @(
    foreach ($candidatePath in @($candidateData.Keys | Sort-Object)) {
        $candidate = $candidateData[$candidatePath]
        $markers = @($candidate.markers | Sort-Object)
        $commandCandidates = @()
        $candidateAmbiguities = @()

        if ($null -ne $candidate.manifestSurface) {
            $manifestCommands = Get-ManifestCommandCandidates $candidate.manifestSurface
            $commandCandidates = @($manifestCommands.commands)
            if ($manifestCommands.issues.Count -gt 0) {
                $candidateAmbiguities += [PSCustomObject]@{
                    path = $candidatePath
                    evidence = @($manifestCommands.issues)
                }
            }
        }
        else {
            if ($markers -contains 'maven') {
                $commandCandidates += New-CommandCandidate -Executable 'mvn' -Arguments @('test')
            }
            if ($markers -contains 'gradle') {
                $commandCandidates += New-CommandCandidate -Executable 'gradle' -Arguments @('test')
            }
            if ($markers -contains 'go') {
                $commandCandidates += New-CommandCandidate -Executable 'go' -Arguments @('test', './...')
            }
            if ($markers -contains 'node') {
                $packageJsonPath = Join-Path $candidate.fullPath 'package.json'
                if (Test-Path -LiteralPath $packageJsonPath -PathType Leaf) {
                    try {
                        $package = Get-Content -LiteralPath $packageJsonPath -Raw | ConvertFrom-Json -Depth 64
                        foreach ($script in @($package.scripts.PSObject.Properties | Sort-Object Name)) {
                            $commandCandidates += New-CommandCandidate -Executable 'npm' -Arguments @('run', [string]$script.Name)
                        }
                    }
                    catch {
                        $candidateAmbiguities += [PSCustomObject]@{
                            path = $candidatePath
                            evidence = @((Get-RelativePath $resolvedRoot $packageJsonPath), 'package.json could not be parsed')
                        }
                    }
                }
            }
        }

        $candidateInstructionFiles = @(
            $instructionFiles | Where-Object {
                $instructionDirectory = Split-Path $_ -Parent
                $instructionDirectory -eq '' -or $candidatePath -eq $instructionDirectory -or $candidatePath.StartsWith("$instructionDirectory/", [System.StringComparison]::OrdinalIgnoreCase)
            }
        )
        $candidateContracts = @(
            if ($candidatePath -eq '.') {
                $contracts
            }
            else {
                $contracts | Where-Object { $_.StartsWith("$candidatePath/", [System.StringComparison]::OrdinalIgnoreCase) }
            }
        )
        $lockFiles = @(
            $files |
                Where-Object { $_.DirectoryName -eq $candidate.fullPath -and $_.Name -in $nodeLockFiles } |
                ForEach-Object { Get-RelativePath $resolvedRoot $_.FullName } |
                Sort-Object
        )
        if (($markers -contains 'node') -and $lockFiles.Count -gt 1) {
            $candidateAmbiguities += [PSCustomObject]@{
                path = $candidatePath
                evidence = $lockFiles
            }
        }

        [PSCustomObject]@{
            path = $candidatePath
            markers = $markers
            instructionFiles = $candidateInstructionFiles
            contracts = $candidateContracts
            commandCandidates = @($commandCandidates)
            ambiguities = @($candidateAmbiguities)
        }
    }
)

$ambiguities = @($manifestIssues)
foreach ($candidate in $candidates) {
    $ambiguities += @($candidate.ambiguities)
}

$result = [PSCustomObject]@{
    schemaVersion = 1
    repoRoot = ConvertTo-NormalizedPath $resolvedRoot
    status = if ($ambiguities.Count -gt 0) { 'blocked' } else { 'pass' }
    instructionFiles = $instructionFiles
    contracts = $contracts
    candidates = $candidates
    ambiguities = $ambiguities
}

$json = $result | ConvertTo-Json -Depth 64
if (-not [string]::IsNullOrWhiteSpace($OutputPath)) {
    $outputDirectory = Split-Path -Parent $OutputPath
    if (-not [string]::IsNullOrWhiteSpace($outputDirectory)) {
        New-Item -ItemType Directory -Path $outputDirectory -Force | Out-Null
    }
    Set-Content -LiteralPath $OutputPath -Value $json -Encoding utf8NoBOM
}

Write-Output $json
