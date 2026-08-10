$ErrorActionPreference = 'Stop'

$skillRoot = Split-Path $PSScriptRoot -Parent
$verify = Join-Path $skillRoot 'scripts/verify.ps1'
$fixture = Join-Path $PSScriptRoot 'fixtures/verification'
$manifestPath = Join-Path $fixture 'project.json'
$resultsPath = Join-Path $fixture 'test-results.json'
$resultsTemplatePath = Join-Path $fixture 'test-results-template.json'
$reportsPath = Join-Path $fixture 'harness/reports'
$resultLinkPath = Join-Path $fixture 'result-link-dir'
$outsideResultsPath = Join-Path (Split-Path $fixture -Parent) 'verification-outside-results'

function Assert-Equal($Actual, $Expected, [string]$Message) {
    if ($Actual -ne $Expected) {
        throw "$Message. Expected '$Expected', got '$Actual'."
    }
}

function Assert-True($Condition, [string]$Message) {
    if (-not $Condition) {
        throw $Message
    }
}

function Assert-HasProperty($Value, [string]$Name, [string]$Context) {
    Assert-True ($null -ne $Value.PSObject.Properties[$Name]) "$Context must contain '$Name'."
}

function Get-Gates($Report) {
    if ($Report.gates -is [array]) {
        return @($Report.gates)
    }

    $gates = New-Object System.Collections.ArrayList
    foreach ($property in @($Report.gates.PSObject.Properties)) {
        $gate = $property.Value
        if ($null -ne $gate -and $null -eq $gate.PSObject.Properties['gateId']) {
            $gate | Add-Member -NotePropertyName gateId -NotePropertyValue $property.Name
        }
        [void]$gates.Add($gate)
    }
    return @($gates)
}

function Get-Gate($Report, [string]$GateId) {
    $gate = @(Get-Gates $Report | Where-Object { $_.gateId -eq $GateId })
    Assert-Equal $gate.Count 1 "gate '$GateId' count"
    return $gate[0]
}

function Read-LatestReport {
    $latestPath = Join-Path $reportsPath 'latest.json'
    Assert-True (Test-Path -LiteralPath $latestPath) 'latest.json was not written.'
    return Get-Content -LiteralPath $latestPath -Raw | ConvertFrom-Json
}

function Remove-TestDirectory {
    param([Parameter(Mandatory)][string]$Path)

    if (-not (Test-Path -LiteralPath $Path)) {
        return
    }
    $item = Get-Item -LiteralPath $Path -Force
    if (($item.Attributes -band [System.IO.FileAttributes]::ReparsePoint) -ne 0) {
        Remove-Item -LiteralPath $Path -Force
    }
    else {
        Remove-Item -LiteralPath $Path -Recurse -Force
    }
}

function Invoke-Verifier {
    param(
        [string[]]$Arguments,
        [int]$ExpectedExitCode = 0
    )

    $output = @(& pwsh -NoProfile -File $verify @Arguments 2>&1)
    Assert-Equal $LASTEXITCODE $ExpectedExitCode "verifier exit code ($($Arguments -join ' ')); output: $($output -join "`n")"
    return ($output -join [Environment]::NewLine) | ConvertFrom-Json -Depth 64
}

$parallelProcesses = [System.Collections.ArrayList]::new()
try {
    Assert-True (Test-Path -LiteralPath $manifestPath) 'verification project fixture is missing.'
    Assert-True (Test-Path -LiteralPath (Join-Path $fixture 'go.mod')) 'verification Go marker fixture is missing.'
    Assert-True (Test-Path -LiteralPath $resultsPath) 'normalized test-result fixture is missing.'
    Assert-True (Test-Path -LiteralPath $resultsTemplatePath) 'normalized test-result template fixture is missing.'

    $project = Get-Content -LiteralPath $manifestPath -Raw | ConvertFrom-Json
    Assert-Equal $project.schemaVersion 1 'verification manifest schema version'
    Assert-True ($project.surfaces.Count -ge 3) 'verification manifest must contain multiple surfaces.'
    foreach ($surface in @($project.surfaces)) {
        Assert-True (-not [string]::IsNullOrWhiteSpace($surface.id)) 'every verification surface needs an id.'
        foreach ($check in @($surface.checks.PSObject.Properties)) {
            Assert-True ($check.Value.arguments -is [array]) "check '$($surface.id):$($check.Name)' arguments must be an array."
            Assert-True ($check.Value.executable -is [string]) "check '$($surface.id):$($check.Name)' executable must be a string."
            Assert-True ([int]$check.Value.timeoutSec -gt 0) "check '$($surface.id):$($check.Name)' timeout must be positive."
        }
    }

    $normalized = Get-Content -LiteralPath $resultsPath -Raw | ConvertFrom-Json
    Assert-Equal $normalized.schemaVersion 1 'normalized result schema version'
    Assert-True ($normalized.tests.Count -eq 3) 'normalized result test count'
    foreach ($test in @($normalized.tests)) {
        foreach ($property in @('name', 'kind', 'status', 'durationSec')) {
            Assert-HasProperty $test $property 'normalized test result'
        }
    }

    if (-not (Test-Path -LiteralPath $verify)) {
        throw "Canonical verification engine is missing: $verify"
    }
    $parseTokens = $null
    $parseErrors = $null
    $engineAst = [System.Management.Automation.Language.Parser]::ParseFile($verify, [ref]$parseTokens, [ref]$parseErrors)
    Assert-Equal $parseErrors.Count 0 'verification engine parse error count'
    $unboundedWaits = @($engineAst.FindAll({
        param($node)
        $node -is [System.Management.Automation.Language.InvokeMemberExpressionAst] -and
            $node.Member.Value -eq 'WaitForExit' -and
            $node.Arguments.Count -eq 0
    }, $true))
    Assert-Equal $unboundedWaits.Count 0 'verification engine unbounded WaitForExit call count'
    $treeKillCalls = @($engineAst.FindAll({
        param($node)
        $node -is [System.Management.Automation.Language.InvokeMemberExpressionAst] -and
            $node.Member.Value -eq 'Kill' -and
            $node.Extent.Text -match '\$true'
    }, $true))
    Assert-True ($treeKillCalls.Count -ge 2) 'verification engine must retry process-tree termination before root-only fallback.'

    Remove-Item -LiteralPath $reportsPath -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath (Join-Path $fixture 'timeout-descendant.marker') -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath (Join-Path $fixture 'dry-run.marker') -Force -ErrorAction SilentlyContinue
    Remove-TestDirectory -Path $resultLinkPath
    Remove-TestDirectory -Path $outsideResultsPath

    $all = Invoke-Verifier @('-Stage', 'surface', '-RepoRoot', $fixture, '-ManifestPath', $manifestPath) 1
    Assert-Equal $all.schemaVersion 1 'latest report schema version'
    foreach ($property in @('runId', 'stage', 'startedAt', 'overall', 'gates')) {
        Assert-HasProperty $all $property 'latest.json'
    }
    Assert-Equal $all.stage 'surface' 'latest report stage'
    Assert-True ($all.runId -match '^\d{8}-\d{6}-\d{3}-[a-f0-9]{8}$') 'latest report runId must use the unique readable timestamp format.'

    $pass = Get-Gate $all 'surface:sample:pass'
    Assert-Equal $pass.status 'pass' 'passing command status'
    Assert-Equal $pass.exitCode 0 'passing command exit code'
    Assert-True ($pass.command.arguments -is [array]) 'gate command arguments must remain structured.'

    $fail = Get-Gate $all 'surface:sample:fail'
    Assert-Equal $fail.status 'fail' 'non-zero command status'
    Assert-Equal $fail.exitCode 7 'non-zero command exact exit code'

    $missing = Get-Gate $all 'surface:sample:missing'
    Assert-Equal $missing.status 'blocked' 'missing executable status'
    Assert-True ($null -eq $missing.exitCode) 'missing executable must not invent an exit code.'

    $timeout = Get-Gate $all 'surface:sample:timeout'
    Assert-Equal $timeout.status 'fail' 'timed-out command status'
    Assert-True ($timeout.reason -match '(?i)tim(e)?out') 'timed-out command must record a timeout reason.'
    Assert-True ([double]$timeout.durationSec -lt 8) 'timed-out command exceeded the bounded termination grace period.'
    Start-Sleep -Seconds 4
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $fixture 'timeout-descendant.marker'))) 'timed-out process tree was not cleaned up.'

    Assert-Equal (Get-Gate $all 'surface:other:fail').exitCode 13 'unfiltered other-surface exit code'

    $latestTestsPath = Join-Path $reportsPath 'latest-tests.json'
    Assert-True (Test-Path -LiteralPath $latestTestsPath) 'latest-tests.json was not written.'
    $latestTests = Get-Content -LiteralPath $latestTestsPath -Raw | ConvertFrom-Json
    Assert-Equal $latestTests.schemaVersion 1 'latest-tests schema version'
    Assert-Equal $latestTests.total 3 'latest-tests total'
    Assert-Equal $latestTests.passed 1 'latest-tests passed count'
    Assert-Equal $latestTests.failed 1 'latest-tests failed count'
    Assert-Equal $latestTests.skipped 1 'latest-tests skipped count'
    Assert-Equal (ConvertTo-Json $latestTests.tests -Compress) (ConvertTo-Json $normalized.tests -Compress) 'latest-tests must copy normalized results exactly.'

    $filtered = Invoke-Verifier @('-Stage', 'surface', '-Surface', 'sample', '-RepoRoot', $fixture, '-ManifestPath', $manifestPath) 1
    $filteredGates = @(Get-Gates $filtered)
    $expectedFilteredGateIds = @(
        'surface:sample:fail',
        'surface:sample:missing',
        'surface:sample:pass',
        'surface:sample:timeout'
    )
    Assert-Equal $filteredGates.Count $expectedFilteredGateIds.Count 'filtered gate count'
    Assert-Equal (($filteredGates | ForEach-Object { $_.gateId } | Sort-Object) -join '|') (($expectedFilteredGateIds | Sort-Object) -join '|') 'filtered gate set'
    Assert-True (($filteredGates | Where-Object { $_.surfaceId -ne 'sample' }).Count -eq 0) 'every filtered gate must belong to the selected surface.'
    Assert-Equal (Get-Gate $filtered 'surface:sample:fail').exitCode 7 'filtered sample exit code'

    $skipped = Invoke-Verifier @('-Stage', 'e2e', '-Surface', 'sample', '-RepoRoot', $fixture, '-ManifestPath', $manifestPath)
    Assert-Equal $skipped.overall 'skipped' 'inapplicable stage overall status'
    Assert-True ((Get-Gates $skipped | Where-Object { $_.status -eq 'skipped' }).Count -gt 0) 'inapplicable stage must be reported as skipped.'

    $dryRunMissing = Invoke-Verifier @('-Stage', 'surface', '-Surface', 'sample', '-RepoRoot', $fixture, '-ManifestPath', $manifestPath, '-DryRun') 2
    Assert-Equal $dryRunMissing.overall 'blocked' 'dry-run missing-executable overall status'
    Assert-Equal (Get-Gate $dryRunMissing 'surface:sample:missing').status 'blocked' 'dry-run missing executable status'
    Assert-Equal (Get-Gate $dryRunMissing 'surface:sample:pass').status 'skipped' 'dry-run available executable status'

    $collision = Invoke-Verifier @('-Stage', 'surface', '-Surface', 'collision', '-RepoRoot', $fixture, '-ManifestPath', $manifestPath, '-DryRun')
    $collisionGates = @(Get-Gates $collision)
    Assert-Equal $collisionGates.Count 2 'colliding explicit command/check count'
    Assert-Equal @($collisionGates | Select-Object -ExpandProperty gateId -Unique).Count 2 'colliding explicit command/check unique gate count'
    Assert-Equal (($collisionGates | ForEach-Object { $_.command.arguments[-1] } | Sort-Object) -join '|') ((@("Write-Output 'candidate command'", "Write-Output 'named check'") | Sort-Object) -join '|') 'colliding explicit command/check preservation'

    $staleResults = Invoke-Verifier @('-Stage', 'integration', '-Surface', 'stale-results', '-RepoRoot', $fixture, '-ManifestPath', $manifestPath) 2
    $staleGate = Get-Gate $staleResults 'integration:stale-results:stale'
    Assert-Equal $staleGate.status 'blocked' 'stale normalized result status'
    Assert-Equal $staleGate.exitCode 0 'stale normalized result exact command exit code'
    Assert-True ($staleGate.reason -match '(?i)produced|replaced|fresh') 'stale normalized result must explain missing execution evidence.'
    $staleLatestTests = Get-Content -LiteralPath (Join-Path $reportsPath 'latest-tests.json') -Raw | ConvertFrom-Json
    Assert-Equal $staleLatestTests.total 0 'stale normalized result must not be copied'

    New-Item -ItemType Directory -Path $resultLinkPath -Force | Out-Null
    New-Item -ItemType Directory -Path $outsideResultsPath -Force | Out-Null
    Copy-Item -LiteralPath $resultsTemplatePath -Destination (Join-Path $resultLinkPath 'test-results.json') -Force
    Copy-Item -LiteralPath $resultsTemplatePath -Destination (Join-Path $outsideResultsPath 'test-results.json') -Force
    $resultSwap = Invoke-Verifier @('-Stage', 'integration', '-Surface', 'result-swap', '-RepoRoot', $fixture, '-ManifestPath', $manifestPath) 2
    $resultSwapGate = Get-Gate $resultSwap 'integration:result-swap:swap'
    Assert-Equal $resultSwapGate.status 'blocked' 'post-execution result containment status'
    Assert-Equal $resultSwapGate.exitCode 0 'post-execution result containment exact command exit code'
    Assert-True ($resultSwapGate.reason -match '(?i)RepoRoot|contain|changed|safely') 'post-execution result containment reason'
    Assert-Equal @($resultSwap.tests).Count 0 'post-execution escaped results must not be copied'

    $invalidResults = Invoke-Verifier @('-Stage', 'integration', '-Surface', 'invalid-results', '-RepoRoot', $fixture, '-ManifestPath', $manifestPath) 1
    $invalidResultsGate = Get-Gate $invalidResults 'integration:invalid-results:fail'
    Assert-Equal $invalidResultsGate.status 'fail' 'invalid result configuration must preserve command failure status'
    Assert-Equal $invalidResultsGate.exitCode 9 'invalid result configuration must preserve exact command exit code'

    Remove-Item -LiteralPath (Join-Path $fixture 'dry-run.marker') -Force -ErrorAction SilentlyContinue
    $dryRun = Invoke-Verifier @('-Stage', 'surface', '-Surface', 'dryrun', '-RepoRoot', $fixture, '-ManifestPath', $manifestPath, '-DryRun')
    Assert-Equal $dryRun.overall 'skipped' 'dry-run overall status'
    Assert-Equal (Get-Gate $dryRun 'surface:dryrun:execute').status 'skipped' 'dry-run gate status'
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $fixture 'dry-run.marker'))) 'dry-run executed a command.'

    $latestPath = Join-Path $reportsPath 'latest.json'
    $latestTestsPath = Join-Path $reportsPath 'latest-tests.json'
    $latestBeforeFailure = Get-Content -LiteralPath $latestPath -Raw
    $latestTestsBeforeFailure = Get-Content -LiteralPath $latestTestsPath -Raw
    $latestReadLock = [System.IO.File]::Open(
        $latestPath,
        [System.IO.FileMode]::Open,
        [System.IO.FileAccess]::Read,
        [System.IO.FileShare]::Read
    )
    try {
        $publicationFailureOutput = @(& pwsh -NoProfile -File $verify -Stage surface -Surface dryrun `
            -RepoRoot $fixture -ManifestPath $manifestPath -DryRun 2>&1)
        Assert-True ($LASTEXITCODE -ne 0) "forced latest publication failure must exit non-zero; output: $($publicationFailureOutput -join "`n")"
    }
    finally {
        $latestReadLock.Dispose()
    }
    Assert-Equal (Get-Content -LiteralPath $latestPath -Raw) $latestBeforeFailure 'failed publication latest.json rollback'
    Assert-Equal (Get-Content -LiteralPath $latestTestsPath -Raw) $latestTestsBeforeFailure 'failed publication latest-tests.json rollback'
    Assert-Equal @(Get-ChildItem -LiteralPath $reportsPath -File -Force | Where-Object { $_.Name -match '^\.latest.*\.(tmp|backup)$' }).Count 0 'failed publication temporary artifact count'

    foreach ($index in 1..3) {
        $startInfo = [System.Diagnostics.ProcessStartInfo]::new()
        $startInfo.FileName = (Get-Command pwsh -CommandType Application | Select-Object -First 1).Source
        $startInfo.UseShellExecute = $false
        $startInfo.RedirectStandardOutput = $true
        $startInfo.RedirectStandardError = $true
        foreach ($argument in @(
            '-NoProfile', '-File', $verify,
            '-Stage', 'surface', '-Surface', 'dryrun',
            '-RepoRoot', $fixture, '-ManifestPath', $manifestPath, '-DryRun'
        )) {
            [void]$startInfo.ArgumentList.Add($argument)
        }
        $process = [System.Diagnostics.Process]::new()
        $process.StartInfo = $startInfo
        [void]$process.Start()
        [void]$parallelProcesses.Add([PSCustomObject]@{
            process = $process
            output = $process.StandardOutput.ReadToEndAsync()
            error = $process.StandardError.ReadToEndAsync()
        })
    }

    $parallelReports = @()
    foreach ($context in @($parallelProcesses)) {
        if (-not $context.process.WaitForExit(30000)) {
            $context.process.Kill($true)
            throw 'parallel verifier did not finish within 30 seconds.'
        }
        $parallelOutput = $context.output.GetAwaiter().GetResult()
        $parallelError = $context.error.GetAwaiter().GetResult()
        Assert-Equal $context.process.ExitCode 0 "parallel verifier exit code; stderr: $parallelError"
        $parallelReports += $parallelOutput | ConvertFrom-Json -Depth 64
        $context.process.Dispose()
    }
    $parallelProcesses.Clear()

    $parallelRunIds = @($parallelReports | ForEach-Object { $_.runId })
    Assert-Equal @($parallelRunIds | Select-Object -Unique).Count 3 'parallel verifier unique runId count'
    foreach ($parallelReport in $parallelReports) {
        $runReportPath = Join-Path $reportsPath "$($parallelReport.runId)/report.json"
        Assert-True (Test-Path -LiteralPath $runReportPath -PathType Leaf) "parallel run report is missing: $($parallelReport.runId)"
        $runReport = Get-Content -LiteralPath $runReportPath -Raw | ConvertFrom-Json -Depth 64
        Assert-Equal $runReport.runId $parallelReport.runId 'parallel run directory report identity'
    }
    $parallelLatest = Get-Content -LiteralPath (Join-Path $reportsPath 'latest.json') -Raw | ConvertFrom-Json -Depth 64
    $parallelLatestTests = Get-Content -LiteralPath (Join-Path $reportsPath 'latest-tests.json') -Raw | ConvertFrom-Json -Depth 64
    Assert-True ($parallelLatest.runId -in $parallelRunIds) 'latest.json must reference one complete parallel run.'
    Assert-Equal $parallelLatestTests.runId $parallelLatest.runId 'latest report pointers must reference the same complete run.'

    Write-Output 'PASS: verification contract assertions (structured commands, exact statuses, fresh contained results, unique atomic reports, bounded timeout cleanup, filters, dry-run, and schemas).'
}
finally {
    foreach ($context in @($parallelProcesses)) {
        if ($null -ne $context.process) {
            if (-not $context.process.HasExited) {
                $context.process.Kill($true)
                [void]$context.process.WaitForExit(5000)
            }
            $context.process.Dispose()
        }
    }
    Remove-Item -LiteralPath $reportsPath -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath (Join-Path $fixture 'timeout-descendant.marker') -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath (Join-Path $fixture 'dry-run.marker') -Force -ErrorAction SilentlyContinue
    Remove-TestDirectory -Path $resultLinkPath
    Remove-TestDirectory -Path $outsideResultsPath
}
