$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$utf8 = [Text.UTF8Encoding]::new($false)
$baseline = Join-Path $repoRoot 'baseline164'
$oldZip = Join-Path $baseline 'codea-harness-1.6.4-windows-x64-install.zip'
$oldChecklist = Get-Content (Join-Path $baseline 'codea-harness-1.6.4-release-checklist.json') -Raw | ConvertFrom-Json
$oldHead = 'd8327ffe51cecac4a3887366ec847e3aec627923'
if ($oldChecklist.exactHeadSha -cne $oldHead -or [string]$oldChecklist.workflowRunId -ne '34804181203') { throw '1.6.4 baseline provenance mismatch' }
if ((Get-FileHash $oldZip -Algorithm SHA256).Hash.ToLowerInvariant() -cne [string]$oldChecklist.artifacts.install.sha256) { throw '1.6.4 install ZIP hash mismatch' }
$upgradeZip = Join-Path $repoRoot 'codea-harness-1.6.5-windows-x64-upgrade.zip'
$head = (git -C $repoRoot rev-parse HEAD).Trim()
$project = Join-Path $env:RUNNER_TEMP ('Codea 165 upgrade & spaces ' + [guid]::NewGuid().ToString('N'))
$evidence = [ordered]@{ status='PENDING'; exactHeadSha=$head; baselineRunId='34804181203'; baselineHead=$oldHead; fromVersion='1.6.4'; toVersion='1.6.5' }

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
function Assert-Snapshot($Expected,$Actual) {
    if ($Expected.Count -ne $Actual.Count) { throw 'snapshot file count changed' }
    foreach ($key in $Expected.Keys) { if ($Expected[$key] -cne $Actual[$key]) { throw "snapshot changed: $key" } }
}
function Assert-NoTransactionLeaks([string]$Root) {
    $leaks = @(Get-ChildItem $Root -Force | Where-Object { $_.Name -like '.code-harness-stage-*' -or $_.Name -like '.code-harness-backup-*' })
    if ($leaks.Count -ne 0) { throw "upgrade transaction leaked: $($leaks.Name -join ',')" }
}
try {
    Expand-Archive $oldZip $project -Force
    $target = Join-Path $project '.code-harness'
    $oldManifest = Get-Content (Join-Path $target 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
    if ([string]$oldManifest.version -cne '1.6.4' -or [string]$oldManifest.buildCommit -cne $oldHead) { throw 'baseline manifest mismatch' }
    Copy-Item (Join-Path $target 'harness.template.yaml') (Join-Path $target 'harness.yaml')
    foreach ($rel in @('project.md','database.yaml','chains/user.yaml','runs/previous/review.md','tools/user-note.txt')) {
        Write-File (Join-Path $target $rel) "user sentinel $rel`n"
    }
    Write-File (Join-Path $project '.opencode/user-settings.json') '{"preserve":true}'
    $preserved = @{}
    foreach ($rel in @('harness.yaml','project.md','database.yaml','chains/user.yaml','runs/previous/review.md','tools/user-note.txt')) {
        $preserved[$rel] = (Get-FileHash (Join-Path $target $rel) -Algorithm SHA256).Hash
    }
    Expand-Archive $upgradeZip $project -Force
    $source = Join-Path $project '.code-harness-upgrade'
    $newManifest = Get-Content (Join-Path $source 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
    if ([string]$newManifest.version -cne '1.6.5' -or [string]$newManifest.buildCommit -cne $head) { throw 'candidate manifest mismatch' }

    # A user-owned Host edit must stop before any framework mutation.
    $reviewerPath = Join-Path $project '.opencode/agents/reviewer.md'
    $oldReviewerBytes = [IO.File]::ReadAllBytes($reviewerPath)
    Write-File $reviewerPath 'user-modified-reviewer'
    $beforeConflict = Snapshot $project
    Push-Location $project
    try {
        $conflictRaw = & (Join-Path $source 'bin/codea-dcep-tools.exe') upgrade
        $conflictExit = $LASTEXITCODE
    } finally { Pop-Location }
    $conflict = ($conflictRaw | Out-String) | ConvertFrom-Json
    if ($conflictExit -eq 0 -or $conflict.status -cne 'MANUAL_ACTION_REQUIRED') { throw "Host conflict accepted: $conflictRaw" }
    Assert-Snapshot $beforeConflict (Snapshot $project)
    Assert-NoTransactionLeaks $project
    [IO.File]::WriteAllBytes($reviewerPath,$oldReviewerBytes)
    Write-Output 'RELEASE165_PACKAGED_HOST_CONFLICT PASS'

    # Execute the target package's real Windows binary from a business project
    # path containing spaces and a shell metacharacter, including self-consumption.
    Push-Location $project
    try {
        $raw = & (Join-Path $source 'bin/codea-dcep-tools.exe') upgrade
        $upgradeExit = $LASTEXITCODE
    } finally { Pop-Location }
    $result = ($raw | Out-String) | ConvertFrom-Json
    if ($upgradeExit -ne 0 -or $result.status -cne 'UPGRADED' -or $result.fromVersion -cne '1.6.4' -or $result.toVersion -cne '1.6.5') { throw "upgrade failed: $raw" }
    if (Test-Path $source) { throw 'successful upgrade did not consume source package' }
    Assert-NoTransactionLeaks $project
    foreach ($rel in $preserved.Keys) {
        if ((Get-FileHash (Join-Path $target $rel) -Algorithm SHA256).Hash -cne $preserved[$rel]) { throw "project state modified: $rel" }
    }
    if ((Get-Content (Join-Path $project '.opencode/user-settings.json') -Raw) -cne '{"preserve":true}') { throw 'unknown Host config changed' }
    $installed = Get-Content (Join-Path $target 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
    if ($installed.buildCommit -cne $head -or $installed.version -cne '1.6.5') { throw 'installed provenance mismatch' }
    foreach ($property in $newManifest.managedFiles.PSObject.Properties) {
        $hash = (Get-FileHash (Join-Path $target $property.Name) -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($hash -cne [string]$property.Value) { throw "installed framework differs from candidate: $($property.Name)" }
    }
    $hostManifest = $newManifest.hostAgents.reviewer
    $hostPairs = @(@($hostManifest.path,$hostManifest.sha256),@($hostManifest.command,$hostManifest.commandSha256),@($hostManifest.submissionTool,$hostManifest.submissionToolSha256))
    foreach ($pair in $hostPairs) {
        $hash = (Get-FileHash (Join-Path $project $pair[0]) -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($hash -cne [string]$pair[1]) { throw "installed Host differs from candidate: $($pair[0])" }
    }
    & (Join-Path $target 'bin/ast-grep.exe') --version
    if ($LASTEXITCODE -ne 0) { throw 'installed ast-grep failed' }
    Push-Location $project
    try {
        $beginRaw = & (Join-Path $target 'bin/codea-dcep-tools.exe') review begin
        if ($LASTEXITCODE -ne 0) { throw "installed review begin failed: $beginRaw" }
    } finally { Pop-Location }
    $begin = ($beginRaw | Out-String) | ConvertFrom-Json
    if (-not $begin.runId) { throw "review begin missing runId: $beginRaw" }
    Write-Output 'RELEASE165_REAL_164_TO_165_UPGRADE PASS'
    Write-Output 'RELEASE165_INSTALLED_RUNTIME_HOST_STATE PASS'

    # A second copy of exactly the installed version is a genuine no-op.
    Expand-Archive $upgradeZip $project -Force
    $beforeNoop = Snapshot $project
    Push-Location $project
    try {
        $noopRaw = & (Join-Path $source 'bin/codea-dcep-tools.exe') upgrade
        if ($LASTEXITCODE -ne 0) { throw "same-version check failed: $noopRaw" }
    } finally { Pop-Location }
    $noop = ($noopRaw | Out-String) | ConvertFrom-Json
    if ($noop.status -cne 'ALREADY_UP_TO_DATE') { throw "same-version result: $noopRaw" }
    Assert-Snapshot $beforeNoop (Snapshot $project)
    Assert-NoTransactionLeaks $project
    $evidence.status = 'PASS'
    $evidence.upgradeResult = $result
    $evidence.preservedFiles = @($preserved.Keys | Sort-Object)
    $evidence.hostConflict = 'MANUAL_ACTION_REQUIRED; zero mutations'
    $evidence.sameVersion = 'ALREADY_UP_TO_DATE; zero mutations'
    $evidence.installedReviewRunId = $begin.runId
    [IO.File]::WriteAllText((Join-Path $repoRoot 'codea-harness-1.6.5-upgrade-evidence.json'),($evidence | ConvertTo-Json -Depth 12),$utf8)
    Write-Output 'RELEASE165_SAME_VERSION_NOOP PASS'
} finally {
    Remove-Item $project -Recurse -Force -ErrorAction SilentlyContinue
}
