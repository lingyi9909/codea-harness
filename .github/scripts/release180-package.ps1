$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

# Reuse the accepted 1.6.7 release adapter byte-for-byte, changing only the
# target version, then add the two 1.8 primary-review Host resources.
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$source = Join-Path $PSScriptRoot 'release167-package.ps1'
$expectedBlob = '6c7797e3303924310646c29e71b5881307cd7496'
$actualBlob = (& git -C $repoRoot hash-object -- $source).Trim()
if ($LASTEXITCODE -ne 0 -or $actualBlob -ne $expectedBlob) {
    throw "Retained 1.6.7 release adapter changed: $actualBlob"
}
$version = (Get-Content (Join-Path $repoRoot '.code-harness/VERSION') -Raw).Trim()
if ($version -ne '1.8.0') { throw "Expected 1.8.0, got $version" }

$adapted = (Get-Content $source -Raw).Replace('1.6.7', '1.8.0')
$temp = Join-Path $PSScriptRoot ('.release180-retained-' + [guid]::NewGuid().ToString('N') + '.tmp.ps1')
try {
    [IO.File]::WriteAllText($temp, $adapted, [Text.UTF8Encoding]::new($false))
    & pwsh -NoProfile -File $temp
    if ($LASTEXITCODE -ne 0) { throw "retained 1.8.0 package build failed: $LASTEXITCODE" }
} finally {
    Remove-Item $temp -Force -ErrorAction SilentlyContinue
    $global:LASTEXITCODE = 0
}

function Add-PrimaryReview180Host([string]$ZipPath, [string]$Kind) {
    $stage = Join-Path $env:RUNNER_TEMP ('release180-primary-host-' + [guid]::NewGuid().ToString('N'))
    try {
        New-Item -ItemType Directory -Force $stage | Out-Null
        Expand-Archive -Path $ZipPath -DestinationPath $stage -Force
        $rootName = if ($Kind -eq 'install') { '.code-harness' } else { '.code-harness-upgrade' }
        $harnessRoot = Join-Path $stage $rootName
        if (-not (Test-Path $harnessRoot -PathType Container)) { throw "package missing $rootName" }

        $hostRoot = if ($Kind -eq 'install') { $stage } else { Join-Path $harnessRoot 'host' }
        $primaryAgent = Join-Path $hostRoot '.opencode/agents/orchestrator.md'
        $primaryTool = Join-Path $hostRoot '.opencode/tools/codea-review.ts'
        New-Item -ItemType Directory -Force (Split-Path -Parent $primaryAgent) | Out-Null
        New-Item -ItemType Directory -Force (Split-Path -Parent $primaryTool) | Out-Null
        Copy-Item -LiteralPath (Join-Path $repoRoot '.code-harness/agents/orchestrator.md') -Destination $primaryAgent -Force
        Copy-Item -LiteralPath (Join-Path $repoRoot '.code-harness/tools/codea-review.ts') -Destination $primaryTool -Force

        $manifestPath = Join-Path $harnessRoot 'RELEASE-MANIFEST.json'
        $manifest = Get-Content $manifestPath -Raw | ConvertFrom-Json
        $reviewer = $manifest.hostAgents.reviewer
        if ($null -eq $reviewer) { throw "$Kind legacy Host ownership metadata missing" }
        $reviewer | Add-Member -NotePropertyName primaryAgent -NotePropertyValue '.opencode/agents/orchestrator.md' -Force
        $reviewer | Add-Member -NotePropertyName primaryAgentUpgradeSource -NotePropertyValue 'host/.opencode/agents/orchestrator.md' -Force
        $reviewer | Add-Member -NotePropertyName primaryAgentSha256 -NotePropertyValue ((Get-FileHash $primaryAgent -Algorithm SHA256).Hash.ToLowerInvariant()) -Force
        $reviewer | Add-Member -NotePropertyName primaryTool -NotePropertyValue '.opencode/tools/codea-review.ts' -Force
        $reviewer | Add-Member -NotePropertyName primaryToolUpgradeSource -NotePropertyValue 'host/.opencode/tools/codea-review.ts' -Force
        $reviewer | Add-Member -NotePropertyName primaryToolSha256 -NotePropertyValue ((Get-FileHash $primaryTool -Algorithm SHA256).Hash.ToLowerInvariant()) -Force
        [IO.File]::WriteAllText($manifestPath, ($manifest | ConvertTo-Json -Depth 12), [Text.UTF8Encoding]::new($false))

        if ($Kind -eq 'install') {
            $installerPath = Join-Path $stage 'install.ps1'
            if (-not (Test-Path $installerPath -PathType Leaf)) { throw 'install.ps1 missing from retained package' }
            $installer = Get-Content $installerPath -Raw
            $needle = "    '.opencode/commands/harness-review.md'"
            if (-not $installer.Contains($needle)) { throw '1.8 installer Host list insertion point missing' }
            $replacement = "    '.opencode/commands/harness-review.md'," + [Environment]::NewLine +
                "    '.opencode/agents/orchestrator.md'," + [Environment]::NewLine +
                "    '.opencode/tools/codea-review.ts'"
            $installer = $installer.Replace($needle, $replacement)
            [IO.File]::WriteAllText($installerPath, $installer, [Text.UTF8Encoding]::new($false))
        }

        Remove-Item -Force $ZipPath
        if ($Kind -eq 'install') {
            Compress-Archive -Path @($harnessRoot, (Join-Path $stage '.opencode'), (Join-Path $stage 'install.ps1'), (Join-Path $stage 'install.cmd')) -DestinationPath $ZipPath -Force
        } else {
            Compress-Archive -Path $harnessRoot -DestinationPath $ZipPath -Force
        }
    } finally {
        Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue
    }
}

foreach ($kind in @('install','upgrade')) {
    $zip = Join-Path $repoRoot "codea-harness-1.8.0-windows-x64-$kind.zip"
    if (-not (Test-Path $zip -PathType Leaf)) { throw "retained builder did not produce $zip" }
    Add-PrimaryReview180Host $zip $kind
}

Write-Output 'RELEASE180_PACKAGE_BUILD PASS version=1.8.0'
Write-Output 'RELEASE180_PRIMARY_AGENT_HOST PASS path=.opencode/agents/orchestrator.md'
Write-Output 'RELEASE180_PRIMARY_TOOL_HOST PASS path=.opencode/tools/codea-review.ts'
