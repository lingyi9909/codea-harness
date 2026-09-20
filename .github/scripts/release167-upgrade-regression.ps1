$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$utf8 = [Text.UTF8Encoding]::new($false)
$baseline = Join-Path $repoRoot 'baseline166'
$oldZip = Join-Path $baseline 'codea-harness-1.6.6-windows-x64-install.zip'
$oldChecklist = Get-Content (Join-Path $baseline 'codea-harness-1.6.6-release-checklist.json') -Raw | ConvertFrom-Json
$oldHead = 'b2e5e820c5d157405d708fed1be9f1ccaf8f8270'
if ($oldChecklist.exactHeadSha -cne $oldHead -or [string]$oldChecklist.workflowRunId -ne '34928481795') { throw '1.6.6 baseline provenance mismatch' }
if ((Get-FileHash $oldZip -Algorithm SHA256).Hash.ToLowerInvariant() -cne [string]$oldChecklist.artifacts.install.sha256) { throw '1.6.6 install ZIP hash mismatch' }
$upgradeZip = Join-Path $repoRoot 'codea-harness-1.6.7-windows-x64-upgrade.zip'
$head = (git -C $repoRoot rev-parse HEAD).Trim()
$project = Join-Path $env:RUNNER_TEMP ('Codea 167 upgrade & spaces ' + [guid]::NewGuid().ToString('N'))
$evidence = [ordered]@{ status='PENDING'; exactHeadSha=$head; baselineRunId='34928481795'; baselineInstallArtifactId='10380623448'; baselineChecklistArtifactId='10380389520'; baselineHead=$oldHead; fromVersion='1.6.6'; toVersion='1.6.7' }

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
    if ([string]$oldManifest.version -cne '1.6.6' -or [string]$oldManifest.buildCommit -cne $oldHead) { throw 'baseline manifest mismatch' }
    # A configured business project supplies its review base; the raw template
    # deliberately leaves it empty until initialization.
    $config = (Get-Content (Join-Path $target 'harness.template.yaml') -Raw).Replace('baseRef: ""', 'baseRef: origin/main')
    Write-File (Join-Path $target 'harness.yaml') $config
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
    if ([string]$newManifest.version -cne '1.6.7' -or [string]$newManifest.buildCommit -cne $head) { throw 'candidate manifest mismatch' }

    # The changed submission tool must actually be replaced; a user edit to that
    # same resource must stop before any framework mutation.
    $reviewerPath = Join-Path $project '.opencode/tools/codea-reviewer-submit.ts'
    $oldReviewerBytes = [IO.File]::ReadAllBytes($reviewerPath)
    $oldToolHash = (Get-FileHash $reviewerPath -Algorithm SHA256).Hash.ToLowerInvariant()
    if ($oldToolHash -ceq [string]$newManifest.hostAgents.reviewer.submissionToolSha256) { throw '1.6.7 submission tool must differ from the published 1.6.6 tool' }
    Write-File $reviewerPath 'user-modified-submission-tool'
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
    Write-Output 'RELEASE167_PACKAGED_HOST_CONFLICT PASS'

    # Execute the target package's real Windows binary from a business project
    # path containing spaces and a shell metacharacter, including self-consumption.
    Push-Location $project
    try {
        $raw = & (Join-Path $source 'bin/codea-dcep-tools.exe') upgrade
        $upgradeExit = $LASTEXITCODE
    } finally { Pop-Location }
    $result = ($raw | Out-String) | ConvertFrom-Json
    if ($upgradeExit -ne 0 -or $result.status -cne 'UPGRADED' -or $result.fromVersion -cne '1.6.6' -or $result.toVersion -cne '1.6.7') { throw "upgrade failed: $raw" }
    if (Test-Path $source) { throw 'successful upgrade did not consume source package' }
    Assert-NoTransactionLeaks $project
    foreach ($rel in $preserved.Keys) {
        if ((Get-FileHash (Join-Path $target $rel) -Algorithm SHA256).Hash -cne $preserved[$rel]) { throw "project state modified: $rel" }
    }
    if ((Get-Content (Join-Path $project '.opencode/user-settings.json') -Raw) -cne '{"preserve":true}') { throw 'unknown Host config changed' }
    $installed = Get-Content (Join-Path $target 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
    if ($installed.buildCommit -cne $head -or $installed.version -cne '1.6.7') { throw 'installed provenance mismatch' }
    foreach ($property in $newManifest.managedFiles.PSObject.Properties) {
        $hash = (Get-FileHash (Join-Path $target $property.Name) -Algorithm SHA256).Hash.ToLowerInvariant()
        if ($hash -cne [string]$property.Value) { throw "installed framework differs from candidate: $($property.Name)" }
    }
    $hostManifest = $newManifest.hostAgents.reviewer
    $hostPairs = @(@($hostManifest.path,$hostManifest.sha256),@($hostManifest.command,$hostManifest.commandSha256),@($hostManifest.submissionTool,$hostManifest.submissionToolSha256),@($hostManifest.primaryCommand,$hostManifest.primaryCommandSha256))
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
    Write-Output 'RELEASE167_REAL_166_TO_167_UPGRADE PASS'
    Write-Output 'RELEASE167_INSTALLED_RUNTIME_HOST_STATE PASS'

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
    $evidence.submissionTool = [ordered]@{ beforeSha256=$oldToolHash; afterSha256=[string]$hostManifest.submissionToolSha256; changed=$true }
    $evidence.sameVersion = 'ALREADY_UP_TO_DATE; zero mutations'
    $evidence.installedReviewRunId = $begin.runId
    [IO.File]::WriteAllText((Join-Path $repoRoot 'codea-harness-1.6.7-upgrade-evidence.json'),($evidence | ConvertTo-Json -Depth 12),$utf8)
    Write-Output 'RELEASE167_SAME_VERSION_NOOP PASS'
} finally {
    Remove-Item $project -Recurse -Force -ErrorAction SilentlyContinue
}
