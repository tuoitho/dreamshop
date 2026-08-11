[CmdletBinding()]
param(
    [Parameter(Mandatory)]
    [ValidateNotNullOrEmpty()]
    [string]$Title
)

$pattern = '^(feat|fix|docs|refactor|test|build|ci|chore|perf|revert)(\([a-z0-9]+(?:-[a-z0-9]+)*\))?!?: .+$'

if ($Title -cnotmatch $pattern) {
    Write-Error @"
Invalid pull-request title: $Title
Expected: <type>(<optional-scope>)<optional-breaking-marker>: <description>
Allowed types: feat, fix, docs, refactor, test, build, ci, chore, perf, revert
Examples: feat(order): add checkout validation | fix!: reject reused refresh tokens
"@
    exit 1
}

Write-Host "Valid pull-request title: $Title"
exit 0
