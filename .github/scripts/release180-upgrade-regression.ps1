$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$utf8 = [Text.UTF8Encoding]::new($false)
$baseline = Join-Path $repoRoot 'baseline167'
$oldZip = Join-Path $baseline 'codea-harness-1.6.7-windows-x64-install.zip'
$oldChecklistPath = Join-Path $baseline 'codea-harness-1.6.7-release-checklist.json'
$oldChecklist = Get-Content $oldChecklistPath -Raw | ConvertFrom-Json
$oldHead = '970573a6d2147733b773c0de0c124d9324914ae8'
$oldRun = '34950443217'
$oldInstallArtifact = '10389138981'
$oldChecklistArtifact = '10389298093'
$oldInstallArchiveDigest = 'sha256:73d801e2e20fa00dfd86f5c6dc72975177b84b8c705e75882031b3d41c3b4018'
$oldChecklistArchiveDigest = 'sha256:4222d6047b1eef3e8bfe90f1bc355389d05a40ba20df723e6aa60398aa2a95d6'

if ($oldChecklist.exactHeadSha -cne $oldHead -or [string]$oldChecklist.workflowRunId -ne $oldRun) {
    throw '1.6.7 baseline provenance mismatch'
}
if (-not (Test-Path $oldZip -PathType Leaf)) { throw '1.6.7 baseline install ZIP missing' }
if ((Get-FileHash $oldZip -Algorithm SHA256).Hash.ToLowerInvariant() -cne [string]$oldChecklist.artifacts.install.sha256) {
    throw '1.6.7 install ZIP content hash mismatch'
}

$upgradeZip = Join-Path $repoRoot 'codea-harness-1.8.0-windows-x64-upgrade.zip'
if (-not (Test-Path $upgradeZip -PathType Leaf)) { throw '1.8.0 upgrade ZIP missing' }
$head = (git -C $repoRoot rev-parse HEAD).Trim()
$project = Join-Path $env:RUNNER_TEMP ('Codea 180 空格 & # % ' + [guid]::NewGuid().ToString('N'))

$evidence = [ordered]@{
    schemaVersion = 1
    status = 'PENDING'
    exactHeadSha = $head
    baseline = [ordered]@{
        head = $oldHead
        runId = $oldRun
        installArtifactId = $oldInstallArtifact
        checklistArtifactId = $oldChecklistArtifact
        installArchiveDigest = $oldInstallArchiveDigest
        checklistArchiveDigest = $oldChecklistArchiveDigest
        installZipSha256 = [string]$oldChecklist.artifacts.install.sha256
    }
    fromVersion = '1.6.7'
    toVersion = '1.8.0'
}

function Write-File([string]$Path,[string]$Content) {
    New-Item -ItemType Directory -Force (Split-Path -Parent $Path) | Out-Null
    [IO.File]::WriteAllText($Path,$Content,$utf8)
}

function Snapshot([string]$Root) {
    $result = @{}
    foreach ($file in Get-ChildItem $Root -Recurse -Force -File) {
        $rel = [IO.Path]::GetRelativePath($Root,$file.FullName).Replace('\','/')
        if ($rel.StartsWith('.code-harness-upgrade/',[StringComparison]::Ordinal)) { continue }
        $result[$rel] = (Get-FileHash $file.FullName -Algorithm SHA256).Hash
    }
    return $result
}

function Assert-Snapshot($Expected,$Actual,[string]$Label) {
    if ($Expected.Count -ne $Actual.Count) { throw "$Label snapshot file count changed" }
    foreach ($key in $Expected.Keys) {
        if (-not $Actual.ContainsKey($key) -or $Expected[$key] -cne $Actual[$key]) {
            throw "$Label snapshot changed: $key"
        }
    }
}

function Assert-NoTransactionLeaks([string]$Root) {
    $leaks = @(Get-ChildItem $Root -Force | Where-Object {
        $_.Name -like '.code-harness-stage-*' -or
        $_.Name -like '.code-harness-backup-*' -or
        $_.Name -like '.codea-harness-tools-old-*'
    })
    if ($leaks.Count -ne 0) { throw "upgrade transaction leaked: $($leaks.Name -join ',')" }
}

try {
    Expand-Archive $oldZip $project -Force
    $target = Join-Path $project '.code-harness'
    $oldManifest = Get-Content (Join-Path $target 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
    if ([string]$oldManifest.version -cne '1.6.7' -or [string]$oldManifest.buildCommit -cne $oldHead) {
        throw 'baseline manifest mismatch'
    }

    $config = (Get-Content (Join-Path $target 'harness.template.yaml') -Raw).Replace('baseRef: ""', 'baseRef: origin/main')
    Write-File (Join-Path $target 'harness.yaml') $config
    foreach ($rel in @('project.md','database.yaml','chains/user.yaml','runs/previous/review.md','tools/user-note.txt')) {
        Write-File (Join-Path $target $rel) "user sentinel $rel"
    }
    Write-File (Join-Path $project '.opencode/user-settings.json') '{"preserve":true}'
    $preserved = @{}
    foreach ($rel in @('harness.yaml','project.md','database.yaml','chains/user.yaml','runs/previous/review.md','tools/user-note.txt')) {
        $preserved[$rel] = (Get-FileHash (Join-Path $target $rel) -Algorithm SHA256).Hash
    }

    Expand-Archive $upgradeZip $project -Force
    $source = Join-Path $project '.code-harness-upgrade'
    $newManifest = Get-Content (Join-Path $source 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
    if ([string]$newManifest.version -cne '1.8.0' -or [string]$newManifest.buildCommit -cne $head) {
        throw 'candidate manifest mismatch'
    }
    $hostManifest = $newManifest.hostAgents.reviewer
    foreach ($field in @('primaryCommand','primaryAgent','primaryTool')) {
        if ([string]::IsNullOrWhiteSpace([string]$hostManifest.$field)) { throw "candidate Host metadata missing $field" }
    }

    # 1.6.7 did not own codea-review.ts. A user file at that destination must
    # stop the upgrade before any framework or Host byte is changed.
    $primaryToolPath = Join-Path $project '.opencode/tools/codea-review.ts'
    Write-File $primaryToolPath 'user-owned-codea-review-tool'
    $beforeConflict = Snapshot $project
    Push-Location $project
    try {
        $conflictRaw = & (Join-Path $source 'bin/codea-dcep-tools.exe') upgrade
        $conflictExit = $LASTEXITCODE
    } finally { Pop-Location }
    $conflict = ($conflictRaw | Out-String) | ConvertFrom-Json
    if ($conflictExit -eq 0 -or $conflict.status -cne 'MANUAL_ACTION_REQUIRED') {
        throw "new Host conflict accepted: $conflictRaw"
    }
    Assert-Snapshot $beforeConflict (Snapshot $project) 'Host conflict'
    Assert-NoTransactionLeaks $project
    Remove-Item $primaryToolPath -Force
    Write-Output 'RELEASE180_NEW_HOST_CONFLICT_ZERO_OVERWRITE PASS'

    Push-Location $project
    try {
        $raw = & (Join-Path $source 'bin/codea-dcep-tools.exe') upgrade
        $upgradeExit = $LASTEXITCODE
    } finally { Pop-Location }
    $result = ($raw | Out-String) | ConvertFrom-Json
    if ($upgradeExit -ne 0 -or $result.status -cne 'UPGRADED' -or
        $result.fromVersion -cne '1.6.7' -or $result.toVersion -cne '1.8.0') {
        throw "1.6.7 to 1.8.0 upgrade failed: $raw"
    }
    if (Test-Path $source) { throw 'successful upgrade did not consume source package' }
    Assert-NoTransactionLeaks $project

    foreach ($rel in $preserved.Keys) {
        if ((Get-FileHash (Join-Path $target $rel) -Algorithm SHA256).Hash -cne $preserved[$rel]) {
            throw "project state modified: $rel"
        }
    }
    if ((Get-Content (Join-Path $project '.opencode/user-settings.json') -Raw) -cne '{"preserve":true}') {
        throw 'unknown Host config changed'
    }

    $installed = Get-Content (Join-Path $target 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
    if ([string]$installed.buildCommit -cne $head -or [string]$installed.version -cne '1.8.0') {
        throw 'installed provenance mismatch'
    }
    foreach ($property in $newManifest.managedFiles.PSObject.Properties) {
        $hash = (Get-FileHash (Join-Path $target $property.Name) -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($hash -cne [string]$property.Value) { throw "installed framework differs from candidate: $($property.Name)" }
    }

    $hostPairs = @(
        @($hostManifest.path,$hostManifest.sha256),
        @($hostManifest.command,$hostManifest.commandSha256),
        @($hostManifest.submissionTool,$hostManifest.submissionToolSha256),
        @($hostManifest.primaryCommand,$hostManifest.primaryCommandSha256),
        @($hostManifest.primaryAgent,$hostManifest.primaryAgentSha256),
        @($hostManifest.primaryTool,$hostManifest.primaryToolSha256)
    )
    foreach ($pair in $hostPairs) {
        $hash = (Get-FileHash (Join-Path $project $pair[0]) -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($hash -cne [string]$pair[1]) { throw "installed Host differs from candidate: $($pair[0])" }
    }

    $astVersion = (& (Join-Path $target 'bin/ast-grep.exe') --version | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $astVersion -notmatch '0\.42\.1') { throw "installed ast-grep mismatch: $astVersion" }

    Push-Location $project
    try {
        $startRaw = & (Join-Path $target 'bin/codea-dcep-tools.exe') review start
        $startExit = $LASTEXITCODE
    } finally { Pop-Location }
    if ($startExit -ne 0) { throw "installed review start failed: $startRaw" }
    $start = ($startRaw | Out-String) | ConvertFrom-Json
    if (-not $start.runId -or $start.execution -cne 'INCOMPLETE' -or -not (Test-Path $start.reportPath -PathType Leaf)) {
        throw "installed review start missing durable incomplete report: $startRaw"
    }

    # Same exact release from the same package is a real zero-mutation noop.
    Expand-Archive $upgradeZip $project -Force
    $source = Join-Path $project '.code-harness-upgrade'
    $beforeNoop = Snapshot $project
    Push-Location $project
    try {
        $noopRaw = & (Join-Path $source 'bin/codea-dcep-tools.exe') upgrade
        $noopExit = $LASTEXITCODE
    } finally { Pop-Location }
    if ($noopExit -ne 0) { throw "same-version check failed: $noopRaw" }
    $noop = ($noopRaw | Out-String) | ConvertFrom-Json
    if ($noop.status -cne 'ALREADY_UP_TO_DATE') { throw "same-version result: $noopRaw" }
    Assert-Snapshot $beforeNoop (Snapshot $project) 'Same-version'
    Assert-NoTransactionLeaks $project

    $evidence.status = 'PASS'
    $evidence.upgradeResult = $result
    $evidence.projectPath = $project
    $evidence.preservedFiles = @($preserved.Keys | Sort-Object)
    $evidence.hostConflict = 'MANUAL_ACTION_REQUIRED; zero overwrite'
    $evidence.sameVersion = 'ALREADY_UP_TO_DATE; zero mutations'
    $evidence.installedReviewRunId = [string]$start.runId
    $evidence.installedReviewReportPath = [string]$start.reportPath
    $evidence.astGrepVersion = $astVersion
    [IO.File]::WriteAllText(
        (Join-Path $repoRoot 'codea-harness-1.8.0-upgrade-evidence.json'),
        ($evidence | ConvertTo-Json -Depth 14),
        $utf8
    )
    Write-Output 'RELEASE180_REAL_167_TO_180_UPGRADE PASS'
    Write-Output 'RELEASE180_PROJECT_STATE_PRESERVED PASS'
    Write-Output 'RELEASE180_SAME_VERSION_NOOP PASS'
    Write-Output "RELEASE180_INSTALLED_REVIEW_START PASS runId=$($start.runId)"
} finally {
    Remove-Item $project -Recurse -Force -ErrorAction SilentlyContinue
}
