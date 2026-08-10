#Requires -Version 7
[CmdletBinding()]
param(
    [ValidateSet('discovery', 'contract', 'surface', 'integration', 'e2e', 'all')]
    [string]$Stage = 'all',
    [string[]]$Surface,
    [string]$RepoRoot = (Get-Location).Path,
    [string]$ManifestPath,
    [switch]$DryRun
)

$ErrorActionPreference = 'Stop'
$pathComparison = if ($IsWindows) {
    [System.StringComparison]::OrdinalIgnoreCase
}
else {
    [System.StringComparison]::Ordinal
}

function ConvertTo-NormalizedPath {
    param([Parameter(Mandatory)][string]$Path)

    return $Path.Replace('\', '/')
}

function Resolve-PhysicalPath {
    param(
        [Parameter(Mandatory)][string]$Path,
        [switch]$AllowMissing
    )

    $fullPath = [System.IO.Path]::GetFullPath($Path)
    $pathRoot = [System.IO.Path]::GetPathRoot($fullPath)
    $relativePath = $fullPath.Substring($pathRoot.Length)
    $segments = $relativePath.Split(
        [char[]]@([System.IO.Path]::DirectorySeparatorChar, [System.IO.Path]::AltDirectorySeparatorChar),
        [System.StringSplitOptions]::RemoveEmptyEntries
    )
    $currentPath = $pathRoot

    for ($index = 0; $index -lt $segments.Length; $index++) {
        $candidatePath = Join-Path $currentPath $segments[$index]
        if (-not (Test-Path -LiteralPath $candidatePath)) {
            if (-not $AllowMissing) {
                throw "Path does not exist: $Path"
            }
            for ($remaining = $index; $remaining -lt $segments.Length; $remaining++) {
                $currentPath = Join-Path $currentPath $segments[$remaining]
            }
            break
        }

        $item = Get-Item -LiteralPath $candidatePath -Force
        if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            $target = $item.ResolveLinkTarget($true)
            if ($null -eq $target) {
                throw "Could not resolve link: $($item.FullName)"
            }
            $currentPath = $target.FullName
        }
        else {
            $currentPath = $item.FullName
        }
    }

    return [System.IO.Path]::GetFullPath($currentPath)
}

function Test-ContainedPath {
    param(
        [Parameter(Mandatory)][string]$RootPath,
        [Parameter(Mandatory)][string]$CandidatePath
    )

    $relativePath = [System.IO.Path]::GetRelativePath($RootPath, $CandidatePath)
    return $relativePath -eq '.' -or (
        -not [System.IO.Path]::IsPathRooted($relativePath) -and
        $relativePath -ne '..' -and
        -not $relativePath.StartsWith("..$([System.IO.Path]::DirectorySeparatorChar)", $pathComparison) -and
        -not $relativePath.StartsWith("..$([System.IO.Path]::AltDirectorySeparatorChar)", $pathComparison)
    )
}

function Resolve-ContainedPath {
    param(
        [Parameter(Mandatory)][string]$Path,
        [Parameter(Mandatory)][string]$RootPath,
        [switch]$AllowMissing,
        [ValidateSet('Any', 'Container', 'Leaf')][string]$PathType = 'Any'
    )

    $resolvedPath = Resolve-PhysicalPath -Path $Path -AllowMissing:$AllowMissing
    if (-not (Test-ContainedPath -RootPath $RootPath -CandidatePath $resolvedPath)) {
        throw "Path must resolve within RepoRoot: $Path"
    }
    if (-not $AllowMissing) {
        if ($PathType -eq 'Container' -and -not (Test-Path -LiteralPath $resolvedPath -PathType Container)) {
            throw "Path must be a directory: $Path"
        }
        if ($PathType -eq 'Leaf' -and -not (Test-Path -LiteralPath $resolvedPath -PathType Leaf)) {
            throw "Path must be a file: $Path"
        }
    }

    return $resolvedPath
}

function Get-RelativeReportPath {
    param(
        [Parameter(Mandatory)][string]$RootPath,
        [Parameter(Mandatory)][string]$Path
    )

    return ConvertTo-NormalizedPath ([System.IO.Path]::GetRelativePath($RootPath, $Path))
}

function Get-SafeFileName {
    param([Parameter(Mandatory)][string]$Value)

    $safeValue = $Value -replace '[^A-Za-z0-9._-]', '-'
    $safeValue = $safeValue.Trim('-', '.')
    if ([string]::IsNullOrWhiteSpace($safeValue)) {
        return 'gate'
    }
    return $safeValue
}

function Test-SafeRelativePath {
    param([Parameter(Mandatory)][string]$Path)

    if ([System.IO.Path]::IsPathRooted($Path)) {
        return $false
    }
    $segments = (ConvertTo-NormalizedPath $Path).Split('/', [System.StringSplitOptions]::RemoveEmptyEntries)
    return $segments -notcontains '..'
}

function Resolve-SurfaceDirectory {
    param(
        [Parameter(Mandatory)][string]$SurfacePath,
        [Parameter(Mandatory)][string]$RootPath
    )

    if (-not (Test-SafeRelativePath $SurfacePath)) {
        throw 'surface path must be relative and must not traverse parent directories'
    }
    return Resolve-ContainedPath -Path (Join-Path $RootPath $SurfacePath) -RootPath $RootPath -PathType Container
}

function Resolve-WorkingDirectory {
    param(
        [string]$WorkingDirectory,
        [Parameter(Mandatory)][string]$SurfaceDirectory,
        [Parameter(Mandatory)][string]$RootPath
    )

    if ([string]::IsNullOrWhiteSpace($WorkingDirectory)) {
        return $SurfaceDirectory
    }
    if (-not (Test-SafeRelativePath $WorkingDirectory)) {
        throw 'workingDirectory must be relative and must not traverse parent directories'
    }

    $surfaceCandidate = Join-Path $SurfaceDirectory $WorkingDirectory
    if (Test-Path -LiteralPath $surfaceCandidate -PathType Container) {
        return Resolve-ContainedPath -Path $surfaceCandidate -RootPath $RootPath -PathType Container
    }
    return Resolve-ContainedPath -Path (Join-Path $RootPath $WorkingDirectory) -RootPath $RootPath -PathType Container
}

function Get-CheckStage {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][object]$Check
    )

    if ($null -ne $Check.PSObject.Properties['stage']) {
        $declaredStage = [string]$Check.stage
        if ($declaredStage -in @('contract', 'surface', 'integration', 'e2e')) {
            return $declaredStage
        }
        throw "unsupported check stage: $declaredStage"
    }
    if ($Name -in @('contract', 'integration', 'e2e')) {
        return $Name
    }
    return 'surface'
}

function Get-EnvironmentMap {
    param([object]$Environment)

    $environmentMap = [ordered]@{}
    if ($null -eq $Environment) {
        return $environmentMap
    }
    foreach ($property in @($Environment.PSObject.Properties)) {
        if ([string]::IsNullOrWhiteSpace($property.Name) -or $property.Name.Contains('=') -or $property.Name.Contains([char]0) -or
            $property.Value -isnot [string]) {
            throw 'command environment must be a string-to-string map'
        }
        $environmentMap[$property.Name] = [string]$property.Value
    }
    return $environmentMap
}

function Resolve-Executable {
    param(
        [Parameter(Mandatory)][string]$Executable,
        [Parameter(Mandatory)][string]$WorkingDirectory
    )

    if ([string]::IsNullOrWhiteSpace($Executable)) {
        return $null
    }
    if ([System.IO.Path]::IsPathRooted($Executable) -or $Executable.Contains([System.IO.Path]::DirectorySeparatorChar) -or
        $Executable.Contains([System.IO.Path]::AltDirectorySeparatorChar)) {
        $candidateExecutable = if ([System.IO.Path]::IsPathRooted($Executable)) {
            $Executable
        }
        else {
            Join-Path $WorkingDirectory $Executable
        }
        if (Test-Path -LiteralPath $candidateExecutable -PathType Leaf) {
            return (Resolve-Path -LiteralPath $candidateExecutable).Path
        }
        return $null
    }

    $command = Get-Command -Name $Executable -CommandType Application -ErrorAction SilentlyContinue | Select-Object -First 1
    if ($null -eq $command) {
        return $null
    }
    return $command.Source
}

function Write-GateLog {
    param(
        [Parameter(Mandatory)][string]$Path,
        [string]$StandardOutput,
        [string]$StandardError
    )

    $logParts = @()
    if (-not [string]::IsNullOrEmpty($StandardOutput)) {
        $logParts += $StandardOutput.TrimEnd("`r", "`n")
    }
    if (-not [string]::IsNullOrEmpty($StandardError)) {
        $logParts += $StandardError.TrimEnd("`r", "`n")
    }
    Set-Content -LiteralPath $Path -Value ($logParts -join [Environment]::NewLine) -Encoding utf8NoBOM
}

function Invoke-StructuredCommand {
    param(
        [Parameter(Mandatory)][object]$Command,
        [Parameter(Mandatory)][string]$WorkingDirectory,
        [Parameter(Mandatory)][string]$LogPath
    )

    $resolvedExecutable = Resolve-Executable -Executable ([string]$Command.executable) -WorkingDirectory $WorkingDirectory
    if ($null -eq $resolvedExecutable) {
        Write-GateLog -Path $LogPath -StandardError "Executable was not found: $($Command.executable)"
        return [PSCustomObject]@{
            status = 'blocked'
            exitCode = $null
            durationSec = 0
            reason = "executable was not found: $($Command.executable)"
            executed = $false
        }
    }

    $timeoutSec = if ($null -eq $Command.PSObject.Properties['timeoutSec']) { 300.0 } else { [double]$Command.timeoutSec }
    if ($timeoutSec -le 0 -or [double]::IsNaN($timeoutSec) -or [double]::IsInfinity($timeoutSec)) {
        Write-GateLog -Path $LogPath -StandardError 'timeoutSec must be a positive number'
        return [PSCustomObject]@{
            status = 'blocked'
            exitCode = $null
            durationSec = 0
            reason = 'timeoutSec must be a positive number'
            executed = $false
        }
    }

    $processStartInfo = [System.Diagnostics.ProcessStartInfo]::new()
    $processStartInfo.FileName = $resolvedExecutable
    $processStartInfo.WorkingDirectory = $WorkingDirectory
    $processStartInfo.UseShellExecute = $false
    $processStartInfo.RedirectStandardOutput = $true
    $processStartInfo.RedirectStandardError = $true
    $processStartInfo.CreateNoWindow = $true
    foreach ($argument in @($Command.arguments)) {
        [void]$processStartInfo.ArgumentList.Add([string]$argument)
    }
    foreach ($entry in (Get-EnvironmentMap -Environment $Command.environment).GetEnumerator()) {
        $processStartInfo.Environment[$entry.Key] = $entry.Value
    }

    $process = [System.Diagnostics.Process]::new()
    $process.StartInfo = $processStartInfo
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    try {
        try {
            [void]$process.Start()
        }
        catch {
            $stopwatch.Stop()
            Write-GateLog -Path $LogPath -StandardError $_.Exception.Message
            return [PSCustomObject]@{
                status = 'blocked'
                exitCode = $null
                durationSec = [math]::Round($stopwatch.Elapsed.TotalSeconds, 3)
                reason = "process could not start: $($_.Exception.Message)"
                executed = $false
            }
        }

        $standardOutputTask = $process.StandardOutput.ReadToEndAsync()
        $standardErrorTask = $process.StandardError.ReadToEndAsync()
        $timeoutMilliseconds = [int][math]::Min([int]::MaxValue, [math]::Ceiling($timeoutSec * 1000))
        $completed = $process.WaitForExit($timeoutMilliseconds)
        $timedOut = -not $completed
        $terminationFailure = $null
        if ($timedOut) {
            if (-not $process.HasExited) {
                try {
                    $process.Kill($true)
                }
                catch [System.InvalidOperationException] {
                    if (-not $process.HasExited) {
                        $terminationFailure = $_.Exception.Message
                    }
                }
                catch {
                    $terminationFailure = $_.Exception.Message
                }
            }

            $terminated = $process.HasExited -or $process.WaitForExit(5000)
            if (-not $terminated) {
                try {
                    $process.Kill($true)
                    $terminationFailure = $null
                }
                catch [System.InvalidOperationException] {
                    if (-not $process.HasExited) {
                        $terminationFailure = @($terminationFailure, "process-tree termination retry failed: $($_.Exception.Message)") `
                            | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Join-String -Separator '; '
                    }
                }
                catch {
                    $terminationFailure = @($terminationFailure, "process-tree termination retry failed: $($_.Exception.Message)") `
                        | Where-Object { -not [string]::IsNullOrWhiteSpace($_) } | Join-String -Separator '; '
                }
                $terminated = $process.HasExited -or $process.WaitForExit(1000)
            }
            if (-not $terminated) {
                if ([string]::IsNullOrWhiteSpace($terminationFailure)) {
                    $terminationFailure = 'process tree did not terminate after two bounded termination attempts'
                }
                try {
                    $process.Kill()
                }
                catch [System.InvalidOperationException] {
                    if (-not $process.HasExited) {
                        $terminationFailure = "$terminationFailure; fallback termination failed: $($_.Exception.Message)"
                    }
                }
                catch {
                    $terminationFailure = "$terminationFailure; fallback termination failed: $($_.Exception.Message)"
                }
                $terminated = $process.HasExited -or $process.WaitForExit(1000)
            }

            $streamsCompleted = [System.Threading.Tasks.Task]::WaitAll(
                [System.Threading.Tasks.Task[]]@($standardOutputTask, $standardErrorTask),
                1000
            )
            $standardOutput = if ($standardOutputTask.IsCompletedSuccessfully) { $standardOutputTask.Result } else { '' }
            $standardError = if ($standardErrorTask.IsCompletedSuccessfully) { $standardErrorTask.Result } else { '' }
            if (-not $terminated -or -not [string]::IsNullOrWhiteSpace($terminationFailure)) {
                $terminationFailure = if ([string]::IsNullOrWhiteSpace($terminationFailure)) {
                    'process tree termination failed within the bounded grace period'
                }
                else {
                    $terminationFailure
                }
                $standardError = @($standardError, $terminationFailure) -join [Environment]::NewLine
            }
            elseif (-not $streamsCompleted) {
                $standardError = @($standardError, 'process output did not close within the bounded capture grace period') -join [Environment]::NewLine
            }
            $stopwatch.Stop()
            Write-GateLog -Path $LogPath -StandardOutput $standardOutput -StandardError $standardError
            return [PSCustomObject]@{
                status = 'fail'
                exitCode = if ($terminated) { $process.ExitCode } else { $null }
                durationSec = [math]::Round($stopwatch.Elapsed.TotalSeconds, 3)
                reason = if ($terminated -and [string]::IsNullOrWhiteSpace($terminationFailure)) {
                    "command timeout after $timeoutSec seconds"
                }
                else {
                    "command timeout after $timeoutSec seconds; process-tree termination failed: $terminationFailure"
                }
                executed = $true
            }
        }

        [void]$process.WaitForExit(1000)
        [void][System.Threading.Tasks.Task]::WaitAll(
            [System.Threading.Tasks.Task[]]@($standardOutputTask, $standardErrorTask),
            1000
        )
        $standardOutput = if ($standardOutputTask.IsCompletedSuccessfully) { $standardOutputTask.Result } else { '' }
        $standardError = if ($standardErrorTask.IsCompletedSuccessfully) { $standardErrorTask.Result } else { '' }
        $stopwatch.Stop()
        Write-GateLog -Path $LogPath -StandardOutput $standardOutput -StandardError $standardError

        return [PSCustomObject]@{
            status = if ($process.ExitCode -eq 0) { 'pass' } else { 'fail' }
            exitCode = $process.ExitCode
            durationSec = [math]::Round($stopwatch.Elapsed.TotalSeconds, 3)
            reason = if ($process.ExitCode -eq 0) { '' } else { "command exited with code $($process.ExitCode)" }
            executed = $true
        }
    }
    finally {
        $process.Dispose()
    }
}

function New-GateRecord {
    param(
        [Parameter(Mandatory)][string]$GateId,
        [string]$SurfaceId,
        [Parameter(Mandatory)][string]$Status,
        [object]$ExitCode,
        [double]$DurationSec = 0,
        [Parameter(Mandatory)][string]$LogPath,
        [object]$Command,
        [string]$Reason = ''
    )

    return [PSCustomObject][ordered]@{
        gateId = $GateId
        surfaceId = $SurfaceId
        status = $Status
        exitCode = $ExitCode
        durationSec = $DurationSec
        logPath = $LogPath
        command = $Command
        reason = $Reason
    }
}

function Get-NormalizedResultContext {
    param(
        [Parameter(Mandatory)][object]$TestResults,
        [Parameter(Mandatory)][string]$WorkingDirectory,
        [Parameter(Mandatory)][string]$RootPath,
        [Parameter(Mandatory)][string]$RunPath
    )

    if ([string]$TestResults.format -ne 'harness-tests-v1') {
        throw 'unsupported normalized test result format'
    }
    if ([string]::IsNullOrWhiteSpace([string]$TestResults.path)) {
        throw 'normalized test result path is required'
    }
    $candidatePath = if ([System.IO.Path]::IsPathRooted([string]$TestResults.path)) {
        [string]$TestResults.path
    }
    else {
        Join-Path $WorkingDirectory ([string]$TestResults.path)
    }
    $declaredPath = [System.IO.Path]::GetFullPath($candidatePath)
    $resultPath = Resolve-ContainedPath -Path $declaredPath -RootPath $RootPath -AllowMissing
    [void](Resolve-ContainedPath -Path (Split-Path $resultPath -Parent) -RootPath $RootPath -PathType Container)
    if ((Test-Path -LiteralPath $declaredPath) -and -not (Test-Path -LiteralPath $declaredPath -PathType Leaf)) {
        throw "normalized test result path must be a file: $($TestResults.path)"
    }
    if (Test-Path -LiteralPath $declaredPath -PathType Leaf) {
        $item = Get-Item -LiteralPath $declaredPath -Force
        if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw 'normalized test result file must not be a reparse point or symbolic link'
        }
    }

    $quarantinePath = $null
    if (Test-Path -LiteralPath $resultPath -PathType Leaf) {
        $quarantinePath = Join-Path $RunPath ('.stale-result.{0}.tmp' -f [guid]::NewGuid().ToString('N'))
        [System.IO.File]::Move($resultPath, $quarantinePath)
    }
    return [PSCustomObject]@{
        declaredPath = $declaredPath
        expectedPhysicalPath = $resultPath
        quarantinePath = $quarantinePath
        rootPath = $RootPath
        produced = $false
    }
}

function Set-NormalizedResultFailure {
    param(
        [Parameter(Mandatory)][object]$Gate,
        [Parameter(Mandatory)][string]$Message
    )

    if ($Gate.status -ne 'fail') {
        $Gate.status = 'blocked'
    }
    $prefix = if ([string]::IsNullOrWhiteSpace([string]$Gate.reason)) { '' } else { "$($Gate.reason); " }
    $Gate.reason = "${prefix}normalized test results unavailable: $Message"
}

function Complete-NormalizedResultContext {
    param([Parameter(Mandatory)][object]$ResultContext)

    if ([string]::IsNullOrWhiteSpace([string]$ResultContext.quarantinePath) -or
        -not (Test-Path -LiteralPath $ResultContext.quarantinePath -PathType Leaf)) {
        return
    }
    if ($ResultContext.produced -or (Test-Path -LiteralPath $ResultContext.expectedPhysicalPath -PathType Leaf)) {
        Remove-Item -LiteralPath $ResultContext.quarantinePath -Force
        return
    }

    $expectedParent = Split-Path $ResultContext.expectedPhysicalPath -Parent
    try {
        $resolvedParent = Resolve-ContainedPath -Path $expectedParent -RootPath $ResultContext.rootPath -PathType Container
        if (-not $resolvedParent.Equals($expectedParent, $pathComparison)) {
            return
        }
        [System.IO.File]::Move($ResultContext.quarantinePath, $ResultContext.expectedPhysicalPath)
    }
    catch {
        # The quarantined stale artifact remains in the contained run directory when its original parent is no longer safe.
    }
}

function Add-NormalizedResults {
    param(
        [Parameter(Mandatory)][object]$Gate,
        [Parameter(Mandatory)][object]$ResultContext,
        [Parameter(Mandatory)][AllowEmptyCollection()][System.Collections.ArrayList]$Results
    )

    try {
        $resultPath = Resolve-ContainedPath -Path $ResultContext.declaredPath -RootPath $ResultContext.rootPath -PathType Leaf
        if (-not $resultPath.Equals([string]$ResultContext.expectedPhysicalPath, $pathComparison)) {
            throw 'normalized test result path changed physical identity during execution'
        }
        $ResultContext.produced = $true
        $document = Get-Content -LiteralPath $resultPath -Raw | ConvertFrom-Json -Depth 64
        if ([int]$document.schemaVersion -ne 1 -or $document.tests -isnot [array]) {
            throw 'normalized test result document must use schemaVersion 1 and contain a tests array'
        }

        $validatedTests = @()
        foreach ($test in @($document.tests)) {
            if ([string]$test.status -notin @('pass', 'fail', 'skip')) {
                throw "unsupported machine test status: $($test.status)"
            }
            $validatedTests += $test
        }
        foreach ($test in $validatedTests) {
            [void]$Results.Add($test)
        }
    }
    catch {
        Set-NormalizedResultFailure -Gate $Gate -Message "result was not safely produced by this execution: $($_.Exception.Message)"
    }
    finally {
        Complete-NormalizedResultContext -ResultContext $ResultContext
    }
}

function New-ExclusiveRunDirectory {
    param(
        [Parameter(Mandatory)][string]$ReportsPath,
        [Parameter(Mandatory)][string]$RootPath
    )

    foreach ($attempt in 1..100) {
        $runId = '{0}-{1}' -f (Get-Date -Format 'yyyyMMdd-HHmmss-fff'), ([guid]::NewGuid().ToString('N').Substring(0, 8))
        $reservationPath = Join-Path $ReportsPath ".$runId.reserve"
        $reservation = $null
        try {
            $reservation = [System.IO.File]::Open(
                $reservationPath,
                [System.IO.FileMode]::CreateNew,
                [System.IO.FileAccess]::Write,
                [System.IO.FileShare]::None
            )
        }
        catch [System.IO.IOException] {
            continue
        }

        try {
            $runPath = Resolve-ContainedPath -Path (Join-Path $ReportsPath $runId) -RootPath $RootPath -AllowMissing
            if (Test-Path -LiteralPath $runPath) {
                continue
            }
            [void][System.IO.Directory]::CreateDirectory($runPath)
            $runPath = Resolve-ContainedPath -Path $runPath -RootPath $RootPath -PathType Container
            return [PSCustomObject]@{
                runId = $runId
                path = $runPath
            }
        }
        finally {
            $reservation.Dispose()
            Remove-Item -LiteralPath $reservationPath -Force -ErrorAction SilentlyContinue
        }
    }

    throw 'Could not allocate a unique verification run directory after 100 attempts.'
}

function New-StagedTextFile {
    param(
        [Parameter(Mandatory)][string]$DestinationPath,
        [Parameter(Mandatory)][string]$Content
    )

    $directory = Split-Path $DestinationPath -Parent
    $fileName = Split-Path $DestinationPath -Leaf
    $temporaryPath = Join-Path $directory ('.{0}.{1}.{2}.tmp' -f $fileName, $PID, [guid]::NewGuid().ToString('N'))
    [System.IO.File]::WriteAllText($temporaryPath, $Content, [System.Text.UTF8Encoding]::new($false))
    return $temporaryPath
}

function Publish-LatestReportPair {
    param(
        [Parameter(Mandatory)][string]$ReportsPath,
        [Parameter(Mandatory)][string]$TestsJson,
        [Parameter(Mandatory)][string]$ReportJson
    )

    $latestTestsPath = Join-Path $ReportsPath 'latest-tests.json'
    $latestReportPath = Join-Path $ReportsPath 'latest.json'
    $temporaryTestsPath = $null
    $temporaryReportPath = $null
    $backupTestsPath = Join-Path $ReportsPath ('.latest-tests.json.{0}.backup' -f [guid]::NewGuid().ToString('N'))
    $backupReportPath = Join-Path $ReportsPath ('.latest.json.{0}.backup' -f [guid]::NewGuid().ToString('N'))
    $testsExisted = Test-Path -LiteralPath $latestTestsPath -PathType Leaf
    $reportExisted = Test-Path -LiteralPath $latestReportPath -PathType Leaf
    $testsPublished = $false
    $reportPublished = $false
    try {
        $temporaryTestsPath = New-StagedTextFile -DestinationPath $latestTestsPath -Content $TestsJson
        $temporaryReportPath = New-StagedTextFile -DestinationPath $latestReportPath -Content $ReportJson
        if ($testsExisted) {
            [System.IO.File]::Copy($latestTestsPath, $backupTestsPath)
        }
        if ($reportExisted) {
            [System.IO.File]::Copy($latestReportPath, $backupReportPath)
        }

        [System.IO.File]::Move($temporaryTestsPath, $latestTestsPath, $true)
        $temporaryTestsPath = $null
        $testsPublished = $true
        [System.IO.File]::Move($temporaryReportPath, $latestReportPath, $true)
        $temporaryReportPath = $null
        $reportPublished = $true
    }
    catch {
        $publicationError = $_.Exception.Message
        $rollbackErrors = @()
        if ($reportPublished) {
            try {
                if ($reportExisted) {
                    [System.IO.File]::Move($backupReportPath, $latestReportPath, $true)
                }
                elseif (Test-Path -LiteralPath $latestReportPath -PathType Leaf) {
                    Remove-Item -LiteralPath $latestReportPath -Force
                }
            }
            catch {
                $rollbackErrors += "latest.json rollback failed: $($_.Exception.Message)"
            }
        }
        if ($testsPublished) {
            try {
                if ($testsExisted) {
                    [System.IO.File]::Move($backupTestsPath, $latestTestsPath, $true)
                }
                elseif (Test-Path -LiteralPath $latestTestsPath -PathType Leaf) {
                    Remove-Item -LiteralPath $latestTestsPath -Force
                }
            }
            catch {
                $rollbackErrors += "latest-tests.json rollback failed: $($_.Exception.Message)"
            }
        }
        $rollbackSuffix = if ($rollbackErrors.Count -eq 0) { '' } else { "; $($rollbackErrors -join '; ')" }
        throw "Atomic latest report publication failed: $publicationError$rollbackSuffix"
    }
    finally {
        foreach ($temporaryArtifact in @($temporaryTestsPath, $temporaryReportPath, $backupTestsPath, $backupReportPath)) {
            if (-not [string]::IsNullOrWhiteSpace($temporaryArtifact)) {
                Remove-Item -LiteralPath $temporaryArtifact -Force -ErrorAction SilentlyContinue
            }
        }
    }
}

function Enter-LatestPublishLock {
    param(
        [Parameter(Mandatory)][string]$ReportsPath,
        [int]$TimeoutMilliseconds = 10000
    )

    $lockPath = Join-Path $ReportsPath '.latest.lock'
    $stopwatch = [System.Diagnostics.Stopwatch]::StartNew()
    while ($stopwatch.ElapsedMilliseconds -lt $TimeoutMilliseconds) {
        try {
            return [System.IO.File]::Open(
                $lockPath,
                [System.IO.FileMode]::OpenOrCreate,
                [System.IO.FileAccess]::ReadWrite,
                [System.IO.FileShare]::None
            )
        }
        catch [System.IO.IOException] {
            Start-Sleep -Milliseconds 10
        }
    }
    throw "Timed out waiting $TimeoutMilliseconds milliseconds to publish latest verification reports."
}

$rootItem = Get-Item -LiteralPath $RepoRoot -Force -ErrorAction Stop
if (-not $rootItem.PSIsContainer) {
    throw "RepoRoot must be a directory: $RepoRoot"
}
$resolvedRoot = Resolve-PhysicalPath -Path $rootItem.FullName

$reportsPath = Resolve-ContainedPath -Path (Join-Path $resolvedRoot 'harness/reports') -RootPath $resolvedRoot -AllowMissing
New-Item -ItemType Directory -Path $reportsPath -Force | Out-Null
$reportsPath = Resolve-ContainedPath -Path $reportsPath -RootPath $resolvedRoot -PathType Container

$run = New-ExclusiveRunDirectory -ReportsPath $reportsPath -RootPath $resolvedRoot
$runId = $run.runId
$startedAt = (Get-Date).ToString('o')
$runPath = $run.path

$effectiveManifestPath = $null
$manifestError = $null
if ([string]::IsNullOrWhiteSpace($ManifestPath)) {
    $defaultManifestPath = Join-Path $resolvedRoot 'harness/project.json'
    if (Test-Path -LiteralPath $defaultManifestPath -PathType Leaf) {
        try {
            $effectiveManifestPath = Resolve-ContainedPath -Path $defaultManifestPath -RootPath $resolvedRoot -PathType Leaf
        }
        catch {
            $manifestError = $_.Exception.Message
        }
    }
}
else {
    try {
        $manifestCandidatePath = if ([System.IO.Path]::IsPathRooted($ManifestPath)) {
            $ManifestPath
        }
        else {
            Join-Path $resolvedRoot $ManifestPath
        }
        $effectiveManifestPath = Resolve-ContainedPath -Path $manifestCandidatePath -RootPath $resolvedRoot -PathType Leaf
    }
    catch {
        $manifestError = $_.Exception.Message
    }
}

$manifest = $null
if ($null -ne $effectiveManifestPath) {
    try {
        $manifest = Get-Content -LiteralPath $effectiveManifestPath -Raw | ConvertFrom-Json -Depth 64
        if ([int]$manifest.schemaVersion -ne 1 -or $manifest.surfaces -isnot [array]) {
            throw 'manifest must use schemaVersion 1 and contain a surfaces array'
        }
    }
    catch {
        $manifestError = "manifest could not be parsed: $($_.Exception.Message)"
    }
}

$discoverScript = Join-Path $PSScriptRoot 'discover-project.ps1'
$discovery = $null
$discoveryError = $manifestError
if ($null -eq $discoveryError) {
    try {
        if ($null -ne $effectiveManifestPath) {
            $discoveryOutput = @(& $discoverScript -RepoRoot $resolvedRoot -ManifestPath $effectiveManifestPath 2>&1)
        }
        else {
            $discoveryOutput = @(& $discoverScript -RepoRoot $resolvedRoot 2>&1)
        }
        $discovery = ($discoveryOutput -join [Environment]::NewLine) | ConvertFrom-Json -Depth 64
        if ([string]$discovery.status -eq 'blocked') {
            $discoveryError = 'repository discovery reported unresolved ambiguity'
        }
    }
    catch {
        $discoveryError = $_.Exception.Message
    }
}

$gates = [System.Collections.ArrayList]::new()
$normalizedResults = [System.Collections.ArrayList]::new()

if ($Stage -in @('discovery', 'all')) {
    $discoveryLogPath = Join-Path $runPath 'discovery.log'
    $discoveryLog = if ($null -ne $discovery) { $discovery | ConvertTo-Json -Depth 64 } else { $discoveryError }
    Set-Content -LiteralPath $discoveryLogPath -Value $discoveryLog -Encoding utf8NoBOM
    [void]$gates.Add((New-GateRecord -GateId 'discovery' -Status $(if ($null -eq $discoveryError) { 'pass' } else { 'blocked' }) `
        -ExitCode $null -LogPath (Get-RelativeReportPath -RootPath $resolvedRoot -Path $discoveryLogPath) -Command $null -Reason $discoveryError))
}

$selectedSurfaces = @($Surface | Where-Object { -not [string]::IsNullOrWhiteSpace($_) })
$requestedExecutionStages = if ($Stage -eq 'all') { @('contract', 'surface', 'integration', 'e2e') } elseif ($Stage -eq 'discovery') { @() } else { @($Stage) }

if ($requestedExecutionStages.Count -gt 0 -and $null -ne $discoveryError) {
    $blockedLogPath = Join-Path $runPath 'verification-blocked.log'
    Set-Content -LiteralPath $blockedLogPath -Value $discoveryError -Encoding utf8NoBOM
    [void]$gates.Add((New-GateRecord -GateId "$Stage`:discovery" -Status 'blocked' -ExitCode $null `
        -LogPath (Get-RelativeReportPath -RootPath $resolvedRoot -Path $blockedLogPath) -Command $null -Reason $discoveryError))
}
elseif ($requestedExecutionStages.Count -gt 0) {
    $surfaceDefinitions = @()
    if ($null -ne $manifest) {
        foreach ($surfaceDefinition in @($manifest.surfaces)) {
            $manifestChecks = [ordered]@{}
            $candidateIndex = 0
            if ($null -ne $surfaceDefinition.commandCandidates) {
                foreach ($commandCandidate in @($surfaceDefinition.commandCandidates)) {
                    $candidateIndex++
                    $manifestChecks["candidate-$candidateIndex"] = $commandCandidate
                }
            }
            foreach ($checkProperty in @($surfaceDefinition.checks.PSObject.Properties)) {
                $checkName = $checkProperty.Name
                if ($manifestChecks.Contains($checkName)) {
                    $suffix = 1
                    $checkName = "check-$($checkProperty.Name)"
                    while ($manifestChecks.Contains($checkName)) {
                        $suffix++
                        $checkName = "check-$($checkProperty.Name)-$suffix"
                    }
                }
                $manifestChecks[$checkName] = $checkProperty.Value
            }
            $surfaceDefinitions += [PSCustomObject]@{
                id = [string]$surfaceDefinition.id
                path = [string]$surfaceDefinition.path
                checks = [PSCustomObject]$manifestChecks
            }
        }
    }
    else {
        foreach ($candidate in @($discovery.candidates)) {
            $checks = [ordered]@{}
            $commandIndex = 0
            foreach ($command in @($candidate.commandCandidates)) {
                $commandIndex++
                $checks["command-$commandIndex"] = $command
            }
            $surfaceDefinitions += [PSCustomObject]@{
                id = [string]$candidate.path
                path = [string]$candidate.path
                checks = [PSCustomObject]$checks
            }
        }
    }

    $knownSurfaceIds = @($surfaceDefinitions | ForEach-Object { $_.id })
    $unknownSurfaceIds = @($selectedSurfaces | Where-Object { $_ -notin $knownSurfaceIds })
    foreach ($unknownSurfaceId in $unknownSurfaceIds) {
        $logPath = Join-Path $runPath ("surface-{0}-blocked.log" -f (Get-SafeFileName $unknownSurfaceId))
        $reason = "requested surface was not found: $unknownSurfaceId"
        Set-Content -LiteralPath $logPath -Value $reason -Encoding utf8NoBOM
        [void]$gates.Add((New-GateRecord -GateId "surface:$unknownSurfaceId`:selection" -SurfaceId $unknownSurfaceId -Status 'blocked' `
            -ExitCode $null -LogPath (Get-RelativeReportPath -RootPath $resolvedRoot -Path $logPath) -Command $null -Reason $reason))
    }

    foreach ($surfaceDefinition in $surfaceDefinitions) {
        $surfaceId = [string]$surfaceDefinition.id
        if ([string]::IsNullOrWhiteSpace($surfaceId)) {
            continue
        }
        if ($selectedSurfaces.Count -gt 0 -and $surfaceId -notin $selectedSurfaces) {
            continue
        }

        try {
            $surfaceDirectory = Resolve-SurfaceDirectory -SurfacePath $surfaceDefinition.path -RootPath $resolvedRoot
        }
        catch {
            $logPath = Join-Path $runPath ("surface-{0}-blocked.log" -f (Get-SafeFileName $surfaceId))
            Set-Content -LiteralPath $logPath -Value $_.Exception.Message -Encoding utf8NoBOM
            [void]$gates.Add((New-GateRecord -GateId "surface:$surfaceId`:path" -SurfaceId $surfaceId -Status 'blocked' `
                -ExitCode $null -LogPath (Get-RelativeReportPath -RootPath $resolvedRoot -Path $logPath) -Command $null -Reason $_.Exception.Message))
            continue
        }

        $applicableChecks = @()
        foreach ($checkProperty in @($surfaceDefinition.checks.PSObject.Properties)) {
            try {
                $checkStage = Get-CheckStage -Name $checkProperty.Name -Check $checkProperty.Value
                if ($checkStage -in $requestedExecutionStages) {
                    $applicableChecks += [PSCustomObject]@{
                        name = $checkProperty.Name
                        stage = $checkStage
                        command = $checkProperty.Value
                    }
                }
            }
            catch {
                $logPath = Join-Path $runPath ("surface-{0}-{1}-blocked.log" -f (Get-SafeFileName $surfaceId), (Get-SafeFileName $checkProperty.Name))
                Set-Content -LiteralPath $logPath -Value $_.Exception.Message -Encoding utf8NoBOM
                [void]$gates.Add((New-GateRecord -GateId "surface:$surfaceId`:$($checkProperty.Name)" -SurfaceId $surfaceId -Status 'blocked' `
                    -ExitCode $null -LogPath (Get-RelativeReportPath -RootPath $resolvedRoot -Path $logPath) -Command $null -Reason $_.Exception.Message))
            }
        }

        if ($applicableChecks.Count -eq 0 -and $Stage -ne 'all') {
            $logPath = Join-Path $runPath ("{0}-{1}-skipped.log" -f (Get-SafeFileName $Stage), (Get-SafeFileName $surfaceId))
            $reason = "no $Stage check is configured for surface '$surfaceId'"
            Set-Content -LiteralPath $logPath -Value $reason -Encoding utf8NoBOM
            [void]$gates.Add((New-GateRecord -GateId "$Stage`:$surfaceId`:none" -SurfaceId $surfaceId -Status 'skipped' -ExitCode $null `
                -LogPath (Get-RelativeReportPath -RootPath $resolvedRoot -Path $logPath) -Command $null -Reason $reason))
            continue
        }

        foreach ($check in $applicableChecks) {
            $command = $check.command
            $gateId = "$($check.stage):$surfaceId`:$($check.name)"
            $logFileName = "{0}.log" -f (Get-SafeFileName $gateId)
            $logPath = Join-Path $runPath $logFileName
            $reportCommand = [PSCustomObject][ordered]@{
                executable = [string]$command.executable
                arguments = @([string[]]$command.arguments)
            }

            if ([string]::IsNullOrWhiteSpace([string]$command.executable) -or $command.arguments -isnot [array]) {
                $reason = 'command must provide an executable and argument array'
                Write-GateLog -Path $logPath -StandardError $reason
                [void]$gates.Add((New-GateRecord -GateId $gateId -SurfaceId $surfaceId -Status 'blocked' -ExitCode $null `
                    -LogPath (Get-RelativeReportPath -RootPath $resolvedRoot -Path $logPath) -Command $reportCommand -Reason $reason))
                continue
            }

            try {
                foreach ($argument in @($command.arguments)) {
                    if ($argument -isnot [string]) {
                        throw 'command arguments must contain only strings'
                    }
                }
                if ($null -ne $command.PSObject.Properties['timeoutSec']) {
                    if ($command.timeoutSec -isnot [int] -and $command.timeoutSec -isnot [long] -and
                        $command.timeoutSec -isnot [double] -and $command.timeoutSec -isnot [decimal]) {
                        throw 'timeoutSec must be numeric'
                    }
                    $validatedTimeout = [double]$command.timeoutSec
                    if ($validatedTimeout -le 0 -or [double]::IsNaN($validatedTimeout) -or [double]::IsInfinity($validatedTimeout)) {
                        throw 'timeoutSec must be a positive number'
                    }
                }
                $workingDirectory = Resolve-WorkingDirectory -WorkingDirectory ([string]$command.workingDirectory) `
                    -SurfaceDirectory $surfaceDirectory -RootPath $resolvedRoot
                [void](Get-EnvironmentMap -Environment $command.environment)
                if ($null -eq (Resolve-Executable -Executable ([string]$command.executable) -WorkingDirectory $workingDirectory)) {
                    throw "executable was not found: $($command.executable)"
                }
            }
            catch {
                Write-GateLog -Path $logPath -StandardError $_.Exception.Message
                [void]$gates.Add((New-GateRecord -GateId $gateId -SurfaceId $surfaceId -Status 'blocked' -ExitCode $null `
                    -LogPath (Get-RelativeReportPath -RootPath $resolvedRoot -Path $logPath) -Command $reportCommand -Reason $_.Exception.Message))
                continue
            }

            $reportCommand | Add-Member -NotePropertyName workingDirectory -NotePropertyValue (Get-RelativeReportPath -RootPath $resolvedRoot -Path $workingDirectory)
            if ($DryRun) {
                $reason = 'dry-run: command was not executed'
                Write-GateLog -Path $logPath -StandardOutput $reason
                [void]$gates.Add((New-GateRecord -GateId $gateId -SurfaceId $surfaceId -Status 'skipped' -ExitCode $null `
                    -LogPath (Get-RelativeReportPath -RootPath $resolvedRoot -Path $logPath) -Command $reportCommand -Reason $reason))
                continue
            }

            $normalizedResultContext = $null
            $normalizedResultError = $null
            if ($null -ne $command.testResults) {
                try {
                    $normalizedResultContext = Get-NormalizedResultContext -TestResults $command.testResults `
                        -WorkingDirectory $workingDirectory -RootPath $resolvedRoot -RunPath $runPath
                }
                catch {
                    $normalizedResultError = $_.Exception.Message
                }
            }

            $execution = Invoke-StructuredCommand -Command $command -WorkingDirectory $workingDirectory -LogPath $logPath
            $gate = New-GateRecord -GateId $gateId -SurfaceId $surfaceId -Status $execution.status -ExitCode $execution.exitCode `
                -DurationSec $execution.durationSec -LogPath (Get-RelativeReportPath -RootPath $resolvedRoot -Path $logPath) `
                -Command $reportCommand -Reason $execution.reason
            if ($execution.executed -and $null -ne $normalizedResultContext) {
                Add-NormalizedResults -Gate $gate -ResultContext $normalizedResultContext -Results $normalizedResults
            }
            elseif ($null -ne $normalizedResultContext) {
                Complete-NormalizedResultContext -ResultContext $normalizedResultContext
            }
            if (-not [string]::IsNullOrWhiteSpace($normalizedResultError)) {
                Set-NormalizedResultFailure -Gate $gate -Message $normalizedResultError
            }
            [void]$gates.Add($gate)
        }
    }

    if ($surfaceDefinitions.Count -eq 0 -and $gates.Count -eq 0) {
        $logPath = Join-Path $runPath ("$Stage-skipped.log")
        $reason = 'no verification surfaces were discovered'
        Set-Content -LiteralPath $logPath -Value $reason -Encoding utf8NoBOM
        [void]$gates.Add((New-GateRecord -GateId "$Stage`:none" -Status 'skipped' -ExitCode $null `
            -LogPath (Get-RelativeReportPath -RootPath $resolvedRoot -Path $logPath) -Command $null -Reason $reason))
    }
}

$gateArray = @($gates.ToArray())
$testArray = @($normalizedResults.ToArray())
$overall = 'skipped'
foreach ($candidateStatus in @('fail', 'blocked', 'pass', 'skipped')) {
    if (@($gateArray | Where-Object { $_.status -eq $candidateStatus }).Count -gt 0) {
        $overall = $candidateStatus
        break
    }
}

$testsDocument = [PSCustomObject][ordered]@{
    schemaVersion = 1
    runId = $runId
    startedAt = $startedAt
    total = $testArray.Count
    passed = @($testArray | Where-Object { $_.status -eq 'pass' }).Count
    failed = @($testArray | Where-Object { $_.status -eq 'fail' }).Count
    skipped = @($testArray | Where-Object { $_.status -eq 'skip' }).Count
    tests = $testArray
}

$report = [PSCustomObject][ordered]@{
    schemaVersion = 1
    runId = $runId
    stage = $Stage
    startedAt = $startedAt
    overall = $overall
    gates = $gateArray
    tests = $testArray
    discovery = $discovery
}

$testsJson = $testsDocument | ConvertTo-Json -Depth 64
$reportJson = $report | ConvertTo-Json -Depth 64
Set-Content -LiteralPath (Join-Path $runPath 'tests.json') -Value $testsJson -Encoding utf8NoBOM
Set-Content -LiteralPath (Join-Path $runPath 'report.json') -Value $reportJson -Encoding utf8NoBOM
$publishLock = Enter-LatestPublishLock -ReportsPath $reportsPath
try {
    Publish-LatestReportPair -ReportsPath $reportsPath -TestsJson $testsJson -ReportJson $reportJson
}
finally {
    $publishLock.Dispose()
}

Write-Output $reportJson
exit $(if ($overall -eq 'fail') { 1 } elseif ($overall -eq 'blocked') { 2 } else { 0 })
