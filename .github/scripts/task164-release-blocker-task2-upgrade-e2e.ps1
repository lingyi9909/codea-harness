$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$artifactId = 10004022218
$exact163BuildCommit = '2278ad49d4534b57f3144466ffe0e65ddca7f189'
$candidateUpgrade = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-upgrade.zip'
if ([string]::IsNullOrWhiteSpace($env:GH_TOKEN)) { throw 'GH_TOKEN is required to retrieve exact packaged 1.6.3 artifact' }
if (-not (Test-Path $candidateUpgrade -PathType Leaf)) { throw "missing candidate upgrade package: $candidateUpgrade" }
if (-not (Get-Command opencode -ErrorAction SilentlyContinue)) { throw 'pinned OpenCode CLI is required' }

function Expand-Exact163Install([string]$Destination) {
    $outer = Join-Path $env:RUNNER_TEMP ('task164-task2-163-artifact-' + [guid]::NewGuid().ToString('N') + '.zip')
    $artifactDir = Join-Path $env:RUNNER_TEMP ('task164-task2-163-artifact-' + [guid]::NewGuid().ToString('N'))
    try {
        $headers = @{ Authorization="Bearer $env:GH_TOKEN"; Accept='application/vnd.github+json'; 'X-GitHub-Api-Version'='2022-11-28' }
        Invoke-WebRequest -Uri "https://api.github.com/repos/lingyi9909/codea-harness/actions/artifacts/$artifactId/zip" -Headers $headers -OutFile $outer
        Expand-Archive $outer $artifactDir -Force
        $zips = @(Get-ChildItem $artifactDir -File -Filter 'codea-harness-1.6.3-windows-x64-install.zip')
        if ($zips.Count -ne 1) { throw "expected one exact 1.6.3 install ZIP, found $($zips.Count)" }
        New-Item -ItemType Directory -Force $Destination | Out-Null
        Expand-Archive $zips[0].FullName $Destination -Force
    } finally {
        Remove-Item $outer -Force -ErrorAction SilentlyContinue
        Remove-Item $artifactDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Assert-Exact163([string]$Root) {
    $manifest = Get-Content (Join-Path $Root '.code-harness/RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
    $version = (Get-Content (Join-Path $Root '.code-harness/VERSION') -Raw).Trim()
    if ($version -ne '1.6.3' -or [string]$manifest.version -ne '1.6.3' -or [string]$manifest.buildCommit -ne $exact163BuildCommit) {
        throw "unexpected exact 1.6.3 package identity version=$version build=$($manifest.buildCommit)"
    }
}

function Initialize-ProjectState([string]$Root) {
    $target = Join-Path $Root '.code-harness'
    $template = Get-Content (Join-Path $target 'harness.template.yaml') -Raw
    $config = $template.Replace('module: ""', 'module: "task164-upgrade-e2e"').Replace('baseRef: ""', 'baseRef: HEAD')
    if ($config -notmatch '(?m)^\s*baseRef:\s+HEAD\s*$') { throw 'failed to materialize valid 1.6.3 review.baseRef' }
    [IO.File]::WriteAllText((Join-Path $target 'harness.yaml'), $config, [Text.UTF8Encoding]::new($false))
    [IO.File]::WriteAllText((Join-Path $target 'project.md'), "task164-upgrade-e2e`n", [Text.UTF8Encoding]::new($false))
}

function Install-CandidateUpgrade([string]$Root) {
    Expand-Archive -Path $candidateUpgrade -DestinationPath $Root -Force
    if (-not (Test-Path (Join-Path $Root '.code-harness-upgrade/bin/codea-dcep-tools.exe') -PathType Leaf)) { throw 'candidate upgrade Runtime missing' }
}

function Invoke-CandidateUpgrade([string]$Root) {
    Push-Location $Root
    try {
        $ErrorActionPreference = 'Continue'
        $raw = (& ./.code-harness-upgrade/bin/codea-dcep-tools.exe upgrade 2>&1 | Out-String)
        $exit = $LASTEXITCODE
        $ErrorActionPreference = 'Stop'
        return [pscustomobject]@{ ExitCode=$exit; Text=$raw }
    } finally {
        Pop-Location
        $global:LASTEXITCODE = 0
    }
}

function FileHash([string]$Path) { return (Get-FileHash -Algorithm SHA256 $Path).Hash.ToLowerInvariant() }

$success = Join-Path $env:RUNNER_TEMP ('task164-task2-upgrade-success-' + [guid]::NewGuid().ToString('N'))
$rollback = Join-Path $env:RUNNER_TEMP ('task164-task2-upgrade-rollback-' + [guid]::NewGuid().ToString('N'))
$denyApplied = $false
$commandsDir = $null
$principal = [Security.Principal.WindowsIdentity]::GetCurrent().Name
try {
    Expand-Exact163Install $success
    Assert-Exact163 $success
    Initialize-ProjectState $success
    Install-CandidateUpgrade $success
    $result = Invoke-CandidateUpgrade $success
    if ($result.ExitCode -ne 0 -or $result.Text -notmatch '"status"\s*:\s*"UPGRADED"') { throw "official 1.6.3 -> 1.6.4 upgrade failed`n$($result.Text)" }
    if ((Get-Content (Join-Path $success '.code-harness/VERSION') -Raw).Trim() -ne '1.6.4') { throw 'official upgrade did not install VERSION=1.6.4' }
    foreach ($host in @('.opencode/agents/reviewer.md','.opencode/commands/harness-review-reviewer.md','.opencode/tools/codea-reviewer-submit.ts')) {
        if (-not (Test-Path (Join-Path $success $host) -PathType Leaf)) { throw "official upgrade missing Reviewer Host resource $host" }
    }
    Push-Location $success
    try {
        $inventory = (& opencode agent list 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0 -or $inventory -notmatch '(?m)^reviewer\b') { throw "Reviewer not host-resolvable after official upgrade`n$inventory" }
    } finally { Pop-Location }
    Write-Output "TASK164_PACKAGED_163_TO_164_REVIEWER_HOST_UPGRADE PASS artifactId=$artifactId buildCommit=$exact163BuildCommit"

    Expand-Exact163Install $rollback
    Assert-Exact163 $rollback
    Initialize-ProjectState $rollback
    $versionHash = FileHash (Join-Path $rollback '.code-harness/VERSION')
    $manifestHash = FileHash (Join-Path $rollback '.code-harness/RELEASE-MANIFEST.json')
    [IO.File]::WriteAllText((Join-Path $rollback '.task164-user-owned.txt'), "preserve-me`n", [Text.UTF8Encoding]::new($false))
    $userHash = FileHash (Join-Path $rollback '.task164-user-owned.txt')
    New-Item -ItemType Directory -Force (Join-Path $rollback '.opencode/commands') | Out-Null
    $commandsDir = Join-Path $rollback '.opencode/commands'
    & icacls $commandsDir /deny "${principal}:(W)" | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'failed to establish packaged Host write-denial rollback fixture' }
    $denyApplied = $true

    Install-CandidateUpgrade $rollback
    $failed = Invoke-CandidateUpgrade $rollback
    if ($failed.Text -notmatch '"status"\s*:\s*"UPGRADE_FAILED"' -or $failed.Text -notmatch '"rollbackPerformed"\s*:\s*true') {
        throw "packaged Host partial-apply failure did not roll back`n$($failed.Text)"
    }
    & icacls $commandsDir /remove:d "$principal" | Out-Null
    $denyApplied = $false
    if ((FileHash (Join-Path $rollback '.code-harness/VERSION')) -ne $versionHash) { throw 'packaged rollback did not restore 1.6.3 VERSION bytes' }
    if ((FileHash (Join-Path $rollback '.code-harness/RELEASE-MANIFEST.json')) -ne $manifestHash) { throw 'packaged rollback did not restore 1.6.3 manifest bytes' }
    if ((FileHash (Join-Path $rollback '.task164-user-owned.txt')) -ne $userHash) { throw 'packaged rollback changed unknown user file' }
    foreach ($host in @('.opencode/agents/reviewer.md','.opencode/commands/harness-review-reviewer.md','.opencode/tools/codea-reviewer-submit.ts')) {
        if (Test-Path (Join-Path $rollback $host) -PathType Leaf) { throw "packaged rollback leaked partial Reviewer Host resource $host" }
    }
    Write-Output 'TASK164_PACKAGED_163_TO_164_REVIEWER_HOST_ROLLBACK PASS'
} finally {
    if ($denyApplied -and $commandsDir) { & icacls $commandsDir /remove:d "$principal" | Out-Null }
    Remove-Item $success,$rollback -Recurse -Force -ErrorAction SilentlyContinue
}
