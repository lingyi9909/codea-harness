$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Release-only adapter for the already accepted package builder. The accepted
# builder remains unchanged; this adapter changes only the release version and
# adds the OpenCode Reviewer Host resources required by 1.6.4 Task 2.
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$source = Join-Path $PSScriptRoot 'task162-task2-package.ps1'
$expectedBlob = '84ff6bc44c06a3dbabd72cd8f807c008acf05594'
$actualBlob = (& git -C $repoRoot hash-object -- $source).Trim()
if ($LASTEXITCODE -ne 0 -or $actualBlob -ne $expectedBlob) {
    throw "Release builder source changed: $actualBlob"
}
$version = (Get-Content (Join-Path $repoRoot '.code-harness/VERSION') -Raw).Trim()
if ($version -ne '1.6.4') { throw "Expected release version 1.6.4, got $version" }
$text = Get-Content $source -Raw
if ([regex]::Matches($text, [regex]::Escape('1.6.2')).Count -lt 4) {
    throw 'Unexpected retained package builder version contract'
}
$adapted = $text.Replace('1.6.2', '1.6.4')
if ($adapted.Contains('1.6.2')) { throw 'Unadapted release version' }
$temp = Join-Path $PSScriptRoot ('.task164-release-package-' + [guid]::NewGuid().ToString('N') + '.tmp.ps1')
try {
    [IO.File]::WriteAllText($temp, $adapted, [Text.UTF8Encoding]::new($false))
    & pwsh -NoProfile -File $temp
    if ($LASTEXITCODE -ne 0) { throw "Release package builder failed: $LASTEXITCODE" }
} finally {
    Remove-Item $temp -Force -ErrorAction SilentlyContinue
    $global:LASTEXITCODE = 0
}

function New-ReviewerHostAgent([string]$Destination) {
    $reviewerSource = Join-Path $repoRoot '.code-harness/agents/reviewer.md'
    if (-not (Test-Path $reviewerSource -PathType Leaf)) { throw 'canonical Reviewer agent source missing' }
    $canonical = Get-Content $reviewerSource -Raw
    $match = [regex]::Match($canonical, '(?s)^---\r?\n.*?\r?\n---\r?\n(?<body>.*)$')
    if (-not $match.Success) { throw 'canonical Reviewer frontmatter is malformed' }
    $body = $match.Groups['body'].Value
    $hostText = @"
---
description: Codea Harness independent semantic Reviewer. Produces requests-only proposals; Runtime owns certification and report authority.
mode: subagent
permissions:
  - action: edit
    resource: "*"
    effect: deny
  - action: edit
    resource: ".code-harness/runs/*/requests/**"
    effect: allow
  - action: shell
    resource: "*"
    effect: deny
  - action: subagent
    resource: "*"
    effect: deny
---
$body
"@
    $dir = Split-Path -Parent $Destination
    New-Item -ItemType Directory -Force $dir | Out-Null
    [IO.File]::WriteAllText($Destination, $hostText, [Text.UTF8Encoding]::new($false))
}

function New-ReviewerHostCommand([string]$Destination) {
    $command = @'
---
description: Delegate one Codea Harness semantic review phase to the independent Reviewer child session.
agent: reviewer
subagent: true
---

Execute only the requested Codea Harness Reviewer semantic proposal phase.
Treat the following text as the exact parent-provided review input; do not replace Runtime authority:

$ARGUMENTS
'@
    $dir = Split-Path -Parent $Destination
    New-Item -ItemType Directory -Force $dir | Out-Null
    [IO.File]::WriteAllText($Destination, $command, [Text.UTF8Encoding]::new($false))
}

function Add-OpenCodeReviewerRegistration([string]$ZipPath, [string]$Kind) {
    $stage = Join-Path $env:RUNNER_TEMP ('task164-host-registration-' + [guid]::NewGuid().ToString('N'))
    try {
        New-Item -ItemType Directory -Force $stage | Out-Null
        Expand-Archive -Path $ZipPath -DestinationPath $stage -Force
        $harnessRootName = if ($Kind -eq 'install') { '.code-harness' } else { '.code-harness-upgrade' }
        $harnessRoot = Join-Path $stage $harnessRootName
        if (-not (Test-Path $harnessRoot -PathType Container)) { throw "package missing $harnessRootName" }

        # Initial install may place Host files directly at their final project
        # paths. Upgrade must never pre-write project-root .opencode: its Host
        # resources are staged under .code-harness-upgrade/host and committed
        # by Controlled Runtime together with framework files.
        $hostRoot = if ($Kind -eq 'install') { $stage } else { Join-Path $harnessRoot 'host' }
        $reviewerDestination = Join-Path $hostRoot '.opencode/agents/reviewer.md'
        $commandDestination = Join-Path $hostRoot '.opencode/commands/harness-review-reviewer.md'
        New-ReviewerHostAgent $reviewerDestination
        New-ReviewerHostCommand $commandDestination

        $manifestPath = Join-Path $harnessRoot 'RELEASE-MANIFEST.json'
        if (-not (Test-Path $manifestPath -PathType Leaf)) { throw "package missing $harnessRootName/RELEASE-MANIFEST.json" }
        $manifest = Get-Content $manifestPath -Raw | ConvertFrom-Json
        $manifest | Add-Member -NotePropertyName hostAgents -NotePropertyValue ([ordered]@{
            reviewer = [ordered]@{
                host = 'opencode'
                path = '.opencode/agents/reviewer.md'
                mode = 'subagent'
                source = '.code-harness/agents/reviewer.md'
                upgradeSource = 'host/.opencode/agents/reviewer.md'
                sha256 = (Get-FileHash -Algorithm SHA256 $reviewerDestination).Hash.ToLowerInvariant()
                command = '.opencode/commands/harness-review-reviewer.md'
                commandUpgradeSource = 'host/.opencode/commands/harness-review-reviewer.md'
                commandSha256 = (Get-FileHash -Algorithm SHA256 $commandDestination).Hash.ToLowerInvariant()
            }
        }) -Force
        [IO.File]::WriteAllText($manifestPath, ($manifest | ConvertTo-Json -Depth 12), [Text.UTF8Encoding]::new($false))

        Remove-Item -Force $ZipPath
        if ($Kind -eq 'install') {
            Compress-Archive -Path @($harnessRoot, (Join-Path $stage '.opencode')) -DestinationPath $ZipPath -Force
        } else {
            Compress-Archive -Path $harnessRoot -DestinationPath $ZipPath -Force
        }
    } finally {
        Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue
    }
}

foreach ($kind in @('install','upgrade')) {
    $path = Join-Path $repoRoot "codea-harness-1.6.4-windows-x64-$kind.zip"
    if (-not (Test-Path $path -PathType Leaf)) { throw "Missing release package: $path" }
    Add-OpenCodeReviewerRegistration $path $kind
}
Write-Output 'TASK164_RELEASE_PACKAGE_BUILD PASS version=1.6.4'
Write-Output 'REVIEWER_HOST_PACKAGE_REGISTRATION PASS path=.opencode/agents/reviewer.md mode=subagent'
Write-Output 'REVIEWER_HOST_COMMAND_REGISTRATION PASS command=.opencode/commands/harness-review-reviewer.md subagent=true'
Write-Output 'REVIEWER_HOST_UPGRADE_STAGED_TRANSACTION PASS source=.code-harness-upgrade/host/.opencode'
