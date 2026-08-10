$ErrorActionPreference = 'Stop'

$skillRoot = Split-Path $PSScriptRoot -Parent
$verify = Join-Path $skillRoot 'scripts/verify.ps1'
$fixture = Join-Path $PSScriptRoot 'fixtures/verification'
$manifestPath = Join-Path $fixture 'project.json'
$resultsPath = Join-Path $fixture 'test-results.json'
$reportsPath = Join-Path $fixture 'harness/reports'

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

function Invoke-Verifier {
    param(
        [string[]]$Arguments,
        [int]$ExpectedExitCode = 0
    )

    $output = @(& pwsh -NoProfile -File $verify @Arguments 2>&1)
    Assert-Equal $LASTEXITCODE $ExpectedExitCode "verifier exit code ($($Arguments -join ' ')); output: $($output -join "`n")"
    return Read-LatestReport
}

try {
    Assert-True (Test-Path -LiteralPath $manifestPath) 'verification project fixture is missing.'
    Assert-True (Test-Path -LiteralPath (Join-Path $fixture 'go.mod')) 'verification Go marker fixture is missing.'
    Assert-True (Test-Path -LiteralPath $resultsPath) 'normalized test-result fixture is missing.'

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

    Remove-Item -LiteralPath $reportsPath -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath (Join-Path $fixture 'timeout-descendant.marker') -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath (Join-Path $fixture 'dry-run.marker') -Force -ErrorAction SilentlyContinue

    $all = Invoke-Verifier @('-Stage', 'surface', '-RepoRoot', $fixture, '-ManifestPath', $manifestPath) 1
    Assert-Equal $all.schemaVersion 1 'latest report schema version'
    foreach ($property in @('runId', 'stage', 'startedAt', 'overall', 'gates')) {
        Assert-HasProperty $all $property 'latest.json'
    }
    Assert-Equal $all.stage 'surface' 'latest report stage'
    Assert-True ($all.runId -match '^\d{8}-\d{6}$') 'latest report runId must use the stable timestamp format.'

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

    Remove-Item -LiteralPath (Join-Path $fixture 'dry-run.marker') -Force -ErrorAction SilentlyContinue
    $dryRun = Invoke-Verifier @('-Stage', 'surface', '-Surface', 'dryrun', '-RepoRoot', $fixture, '-ManifestPath', $manifestPath, '-DryRun')
    Assert-Equal $dryRun.overall 'skipped' 'dry-run overall status'
    Assert-Equal (Get-Gate $dryRun 'surface:dryrun:execute').status 'skipped' 'dry-run gate status'
    Assert-True (-not (Test-Path -LiteralPath (Join-Path $fixture 'dry-run.marker'))) 'dry-run executed a command.'

    Write-Output 'PASS: verification contract assertions (structured commands, exact statuses and exit codes, timeout cleanup, stage/surface filtering, dry-run, and report/result schemas).'
}
finally {
    Remove-Item -LiteralPath $reportsPath -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath (Join-Path $fixture 'timeout-descendant.marker') -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath (Join-Path $fixture 'dry-run.marker') -Force -ErrorAction SilentlyContinue
}
