param(
    [Parameter(Mandatory=$true)]
    [string]$Destination
)
$ErrorActionPreference = "Stop"
$fixtureRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
if (Test-Path -LiteralPath $Destination) {
    $existing = @(Get-ChildItem -LiteralPath $Destination -Force)
    if ($existing.Count -gt 0) { throw "Destination must be empty: $Destination" }
} else {
    New-Item -ItemType Directory -Path $Destination -Force | Out-Null
}
$Destination = (Resolve-Path -LiteralPath $Destination).Path

function Invoke-FixtureGit {
    param([Parameter(ValueFromRemainingArguments=$true)][string[]]$GitArgs)
    & git -C $Destination @GitArgs
    if ($LASTEXITCODE -ne 0) { throw "git failed: $($GitArgs -join ' ')" }
}
Invoke-FixtureGit init --initial-branch=master
Invoke-FixtureGit config user.name "Codea T1 Fixture"
Invoke-FixtureGit config user.email "codea-t1@example.invalid"

# Only touch the new fixture repository. Never alter any business checkout.
Copy-Item -Path (Join-Path $fixtureRoot "before\*") -Destination $Destination -Recurse -Force
Invoke-FixtureGit add -A
Invoke-FixtureGit commit -m "fixture: safe baseline"
Invoke-FixtureGit checkout -b feature/t1-risk-review
Copy-Item -Path (Join-Path $fixtureRoot "after\*") -Destination $Destination -Recurse -Force
Invoke-FixtureGit add -A
Invoke-FixtureGit commit -m "fixture: introduce service and mapper risks"

Write-Host "Fixture created: $Destination"
Invoke-FixtureGit diff master...HEAD --stat
Invoke-FixtureGit status --short
Write-Host "Open OpenCode in this directory, then run /codea-review"
