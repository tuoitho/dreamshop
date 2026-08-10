$ErrorActionPreference = 'Stop'

$skillRoot = Split-Path $PSScriptRoot -Parent
$discover = Join-Path $skillRoot 'scripts/discover-project.ps1'

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

function Get-EvidencePath($Evidence) {
    if ($Evidence -is [string]) {
        return $Evidence.Replace('\', '/')
    }

    return ([string]$Evidence.path).Replace('\', '/')
}

function Assert-NoSemanticFields($Value, [string]$Context) {
    if ($null -eq $Value) {
        return
    }

    if ($Value -is [System.Management.Automation.PSCustomObject]) {
        foreach ($property in @($Value.PSObject.Properties)) {
            if ($property.Name -in @('role', 'ownership', 'dependsOn')) {
                throw "Mechanical discovery must not infer '$($property.Name)' in $Context."
            }
            Assert-NoSemanticFields $property.Value "$Context.$($property.Name)"
        }
        return
    }

    if ($Value -is [System.Collections.IEnumerable] -and $Value -isnot [string]) {
        foreach ($item in $Value) {
            Assert-NoSemanticFields $item $Context
        }
    }
}

$fixture = Join-Path $PSScriptRoot 'fixtures/java-go-react-microservices'
$result = & $discover -RepoRoot $fixture | ConvertFrom-Json
Assert-NoSemanticFields $result 'discovery result'
Assert-Equal $result.schemaVersion 1 'schema version'
Assert-Equal $result.status 'pass' 'repository status'
Assert-Equal @($result.candidates).Count 3 'candidate count'
Assert-Equal @($result.contracts).Count 2 'contract evidence count'

$instructionPaths = @($result.instructionFiles | ForEach-Object { Get-EvidencePath $_ })
Assert-True ($instructionPaths -contains 'AGENTS.md') 'instruction discovery must include the fixture AGENTS.md file.'

$contractPaths = @($result.contracts | ForEach-Object { Get-EvidencePath $_ })
Assert-True ($contractPaths -contains 'contracts/openapi/order.yaml') 'OpenAPI contract evidence is missing.'
Assert-True ($contractPaths -contains 'contracts/proto/inventory.proto') 'protobuf contract evidence is missing.'

$candidateByPath = @{}
foreach ($candidate in @($result.candidates)) {
    $candidateByPath[(Get-EvidencePath $candidate)] = $candidate
    Assert-NoSemanticFields $candidate "candidate $(Get-EvidencePath $candidate)"
}

Assert-True $candidateByPath.ContainsKey('services/order') 'Maven surface candidate is missing.'
Assert-True $candidateByPath.ContainsKey('services/inventory') 'Go surface candidate is missing.'
Assert-True $candidateByPath.ContainsKey('web') 'Node surface candidate is missing.'

$order = $candidateByPath['services/order']
$inventory = $candidateByPath['services/inventory']
$web = $candidateByPath['web']
Assert-True (@($order.markers) -contains 'maven') 'Maven marker was not discovered.'
Assert-True (@($inventory.markers) -contains 'go') 'Go marker was not discovered.'
Assert-True (@($web.markers) -contains 'node') 'Node marker was not discovered.'

$mavenCommands = @($order.commandCandidates)
Assert-True ($mavenCommands.Count -gt 0) 'Maven candidate must expose a structured command candidate.'
Assert-True (($mavenCommands | Where-Object { $_.executable -eq 'mvn' -and $_.arguments -is [array] }).Count -gt 0) 'Maven command candidate must use executable plus argument array.'

$goCommands = @($inventory.commandCandidates)
Assert-True ($goCommands.Count -gt 0) 'Go candidate must expose a structured command candidate.'
Assert-True (($goCommands | Where-Object { $_.executable -eq 'go' -and $_.arguments -is [array] }).Count -gt 0) 'Go command candidate must use executable plus argument array.'

$nodeCommands = @($web.commandCandidates)
Assert-True ($nodeCommands.Count -ge 3) 'Node package scripts must produce structured command candidates.'
foreach ($command in $nodeCommands) {
    Assert-Equal $command.executable 'npm' 'Node command executable'
    Assert-True ($command.arguments -is [array]) 'Node command arguments must be an array.'
    Assert-True (@($command.arguments).Count -ge 2 -and @($command.arguments)[0] -eq 'run') 'Node command arguments must identify the package script.'
}

$ambiguousFixture = Join-Path $PSScriptRoot 'fixtures/ambiguous-node'
$ambiguous = & $discover -RepoRoot $ambiguousFixture | ConvertFrom-Json
Assert-Equal $ambiguous.status 'blocked' 'conflicting Node lockfile status'
Assert-Equal @($ambiguous.candidates).Count 1 'ambiguous candidate count'
Assert-True (@($ambiguous.ambiguities).Count -gt 0) 'conflicting Node lockfiles must produce ambiguity evidence.'
$ambiguityText = (@($ambiguous.ambiguities) | ConvertTo-Json -Compress)
Assert-True ($ambiguityText -match 'package-lock\.json' -and $ambiguityText -match 'pnpm-lock\.yaml') 'ambiguity evidence must name both conflicting lockfiles.'
Assert-NoSemanticFields $ambiguous 'ambiguous discovery result'

$manifestPath = Join-Path ([System.IO.Path]::GetTempPath()) "discover-project-$([guid]::NewGuid()).json"
try {
    @'
{
  "surfaces": [
    {
      "path": "services/order",
      "commandCandidates": [
        {
          "executable": "mvn",
          "arguments": ["test"],
          "role": "injected",
          "environment": { "dependsOn": "injected" },
          "workingDirectory": "services/order",
          "timeoutSec": 10
        },
        {
          "executable": "mvn",
          "arguments": ["test"],
          "workingDirectory": { "ownership": "injected" },
          "timeoutSec": { "dependsOn": "injected" }
        }
      ]
    },
    {
      "path": "../outside",
      "markers": ["unknown"]
    }
  ]
}
'@ | Set-Content -LiteralPath $manifestPath -Encoding utf8NoBOM

    $manifestResult = & $discover -RepoRoot $fixture -ManifestPath $manifestPath | ConvertFrom-Json
    Assert-Equal $manifestResult.status 'blocked' 'invalid manifest surface status'
    Assert-True (-not (@($manifestResult.candidates | ForEach-Object { Get-EvidencePath $_ }) -contains '../outside')) 'manifest must not create an outside-repository candidate.'
    Assert-NoSemanticFields $manifestResult 'manifest discovery result'
}
finally {
    Remove-Item -LiteralPath $manifestPath -Force -ErrorAction SilentlyContinue
}

$invalidPackageRoot = Join-Path ([System.IO.Path]::GetTempPath()) "discover-project-invalid-package-$([guid]::NewGuid())"
New-Item -ItemType Directory -Path $invalidPackageRoot | Out-Null
try {
    '{ invalid json' | Set-Content -LiteralPath (Join-Path $invalidPackageRoot 'package.json') -Encoding utf8NoBOM
    $invalidPackage = & $discover -RepoRoot $invalidPackageRoot | ConvertFrom-Json
    Assert-Equal $invalidPackage.status 'blocked' 'invalid package JSON status'
    Assert-True (@($invalidPackage.ambiguities).Count -gt 0) 'invalid package JSON must produce ambiguity evidence.'
    Assert-NoSemanticFields $invalidPackage 'invalid package discovery result'
}
finally {
    Remove-Item -LiteralPath $invalidPackageRoot -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Output 'PASS: discovery contract assertions (schema, evidence, markers, commands, semantic-field guard, ambiguity blocking, manifest filtering, malformed package handling).'
