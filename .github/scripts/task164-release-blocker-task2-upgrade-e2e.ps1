$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$artifactId = 10004022218
$artifactName = 'codea-harness-1.6.3-windows-x64-install'
$artifactWorkflowRunId = 34082066818
$artifactWorkflowHead = '10593ca8ae9256341df316655145f5cfc8eaece9'
$exact163BuildCommit = '2278ad49d4534b57f3144466ffe0e65ddca7f189'
$upgradeZip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-upgrade.zip'
$utf8 = [Text.UTF8Encoding]::new($false)

if ([string]::IsNullOrWhiteSpace($env:GH_TOKEN)) { throw 'GH_TOKEN is required to retrieve exact packaged 1.6.3 artifact' }
if (-not (Test-Path $upgradeZip -PathType Leaf)) { throw "missing official 1.6.4 upgrade ZIP: $upgradeZip" }
if (-not (Get-Command opencode -ErrorAction SilentlyContinue)) { throw 'pinned OpenCode CLI is required' }
if (-not $IsWindows) { throw 'Task 2 packaged upgrade E2E requires Windows' }

function Get-FileSHA256([string]$Path) {
    return (Get-FileHash -Algorithm SHA256 -LiteralPath $Path).Hash.ToLowerInvariant()
}

function Get-TreeSnapshot([string]$Root) {
    if (-not (Test-Path $Root -PathType Container)) { return [ordered]@{} }
    $snapshot = [ordered]@{}
    foreach ($directory in @(Get-ChildItem -LiteralPath $Root -Recurse -Force -Directory | Sort-Object FullName)) {
        $rel = [IO.Path]::GetRelativePath($Root, $directory.FullName).Replace('\', '/') + '/'
        $snapshot[$rel] = '<DIR>'
    }
    foreach ($file in @(Get-ChildItem -LiteralPath $Root -Recurse -Force -File | Sort-Object FullName)) {
        $rel = [IO.Path]::GetRelativePath($Root, $file.FullName).Replace('\', '/')
        $snapshot[$rel] = Get-FileSHA256 $file.FullName
    }
    return $snapshot
}

function Get-SnapshotSHA256($Snapshot) {
    $lines = foreach ($entry in $Snapshot.GetEnumerator()) { "$($entry.Key)=$($entry.Value)" }
    $bytes = [Text.Encoding]::UTF8.GetBytes(($lines -join "`n"))
    $sha = [Security.Cryptography.SHA256]::Create()
    try { return ([BitConverter]::ToString($sha.ComputeHash($bytes))).Replace('-', '').ToLowerInvariant() }
    finally { $sha.Dispose() }
}

function Assert-SnapshotEqual([string]$Label, $Expected, $Actual) {
    $expectedJson = $Expected | ConvertTo-Json -Compress
    $actualJson = $Actual | ConvertTo-Json -Compress
    if ($expectedJson -cne $actualJson) {
        throw "$Label file tree changed across rollback:`nexpected=$expectedJson`nactual=$actualJson"
    }
}

function Get-OfficialHead {
    $head = (& git -C $repoRoot rev-parse HEAD 2>&1 | Out-String).Trim()
    if ($LASTEXITCODE -ne 0 -or $head -notmatch '^[0-9a-f]{40}$') { throw "cannot resolve exact candidate HEAD: $head" }
    return $head
}

function Expand-Exact163Install([string]$Destination) {
    $outer = Join-Path $env:RUNNER_TEMP ('task164-task2-163-artifact-' + [guid]::NewGuid().ToString('N') + '.zip')
    $artifactDir = Join-Path $env:RUNNER_TEMP ('task164-task2-163-artifact-' + [guid]::NewGuid().ToString('N'))
    $headers = @{ Authorization="Bearer $env:GH_TOKEN"; Accept='application/vnd.github+json'; 'X-GitHub-Api-Version'='2022-11-28' }
    try {
        $metadata = Invoke-RestMethod -Uri "https://api.github.com/repos/lingyi9909/codea-harness/actions/artifacts/$artifactId" -Headers $headers
        if ([int64]$metadata.id -ne $artifactId -or [string]$metadata.name -ne $artifactName -or
            [int64]$metadata.workflow_run.id -ne $artifactWorkflowRunId -or [string]$metadata.workflow_run.head_sha -ne $artifactWorkflowHead -or
            [bool]$metadata.expired -or [int64]$metadata.size_in_bytes -le 0 -or
            [string]$metadata.archive_download_url -notmatch "/actions/artifacts/$artifactId/zip$") {
            throw "exact 1.6.3 artifact metadata rejected: id=$($metadata.id) name=$($metadata.name) run=$($metadata.workflow_run.id) head=$($metadata.workflow_run.head_sha) expired=$($metadata.expired)"
        }
        Invoke-WebRequest -Uri "https://api.github.com/repos/lingyi9909/codea-harness/actions/artifacts/$artifactId/zip" -Headers $headers -OutFile $outer
        Expand-Archive -LiteralPath $outer -DestinationPath $artifactDir -Force
        $zips = @(Get-ChildItem -LiteralPath $artifactDir -File -Filter 'codea-harness-1.6.3-windows-x64-install.zip')
        if ($zips.Count -ne 1) { throw "expected one exact 1.6.3 install ZIP, found $($zips.Count)" }
        New-Item -ItemType Directory -Force -Path $Destination | Out-Null
        Expand-Archive -LiteralPath $zips[0].FullName -DestinationPath $Destination -Force
    } finally {
        Remove-Item -LiteralPath $outer -Force -ErrorAction SilentlyContinue
        Remove-Item -LiteralPath $artifactDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Assert-Exact163Install([string]$ProjectRoot) {
    $harness = Join-Path $ProjectRoot '.code-harness'
    $manifest = Get-Content -LiteralPath (Join-Path $harness 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
    if ((Get-Content -LiteralPath (Join-Path $harness 'VERSION') -Raw).Trim() -ne '1.6.3') { throw 'official artifact VERSION is not 1.6.3' }
    if ([string]$manifest.version -ne '1.6.3' -or [string]$manifest.buildCommit -ne $exact163BuildCommit) {
        throw "unexpected 1.6.3 manifest identity: version=$($manifest.version) buildCommit=$($manifest.buildCommit)"
    }
    if ([string]$manifest.platform -ne 'windows' -or [string]$manifest.arch -ne 'x64') { throw 'official 1.6.3 artifact platform mismatch' }
}

function Assert-UpgradePackage([string]$ExpectedHead) {
    $stage = Join-Path $env:RUNNER_TEMP ('task164-task2-package-check-' + [guid]::NewGuid().ToString('N'))
    try {
        Expand-Archive -LiteralPath $upgradeZip -DestinationPath $stage -Force
        $root = Join-Path $stage '.code-harness-upgrade'
        $manifest = Get-Content -LiteralPath (Join-Path $root 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
        if ((Get-Content -LiteralPath (Join-Path $root 'VERSION') -Raw).Trim() -ne '1.6.4') { throw 'upgrade package VERSION is not 1.6.4' }
        if ([string]$manifest.version -ne '1.6.4' -or [string]$manifest.buildCommit -ne $ExpectedHead) {
            throw "upgrade manifest is not exact HEAD: version=$($manifest.version) buildCommit=$($manifest.buildCommit) expected=$ExpectedHead"
        }
        if ([string]$manifest.platform -ne 'windows' -or [string]$manifest.arch -ne 'x64') { throw 'upgrade package platform mismatch' }
        if ([string]$manifest.hostAgents.reviewer.path -ne '.opencode/agents/reviewer.md' -or [string]$manifest.hostAgents.reviewer.mode -ne 'subagent') {
            throw 'upgrade manifest does not declare Reviewer Host registration'
        }
        $reviewer = Join-Path $root 'host/.opencode/agents/reviewer.md'
        $command = Join-Path $root 'host/.opencode/commands/harness-review-reviewer.md'
        $tool = Join-Path $root 'host/.opencode/tools/codea-reviewer-submit.ts'
        foreach ($path in @($reviewer,$command,$tool,(Join-Path $root 'bin/codea-dcep-tools.exe'))) {
            if (-not (Test-Path $path -PathType Leaf)) { throw "upgrade package missing staged transaction input: $path" }
        }
        if ([string]$manifest.hostAgents.reviewer.sha256 -ne (Get-FileSHA256 $reviewer) -or
            [string]$manifest.hostAgents.reviewer.commandSha256 -ne (Get-FileSHA256 $command) -or
            [string]$manifest.hostAgents.reviewer.submissionToolSha256 -ne (Get-FileSHA256 $tool)) {
            throw 'upgrade package Reviewer Host manifest hashes do not match staged bytes'
        }
    } finally { Remove-Item -LiteralPath $stage -Recurse -Force -ErrorAction SilentlyContinue }
}

function Initialize-ProjectState([string]$ProjectRoot) {
    $harness = Join-Path $ProjectRoot '.code-harness'
    $template = Get-Content -LiteralPath (Join-Path $harness 'harness.template.yaml') -Raw
    $config = $template.Replace('module: ""', 'module: "task164-upgrade-e2e"').Replace('baseRef: ""', 'baseRef: HEAD')
    if ($config -notmatch '(?m)^\s*baseRef:\s+HEAD\s*$') { throw 'failed to materialize valid 1.6.3 review.baseRef' }
    [IO.File]::WriteAllText((Join-Path $harness 'harness.yaml'), $config, $utf8)
    [IO.File]::WriteAllText((Join-Path $harness 'user-framework-file.txt'), "unknown-framework-file`r`n", $utf8)
    New-Item -ItemType Directory -Force -Path (Join-Path $ProjectRoot '.opencode') | Out-Null
    [IO.File]::WriteAllText((Join-Path $ProjectRoot '.opencode/user-settings.json'), "{`"userOwned`":true}`r`n", $utf8)
}

function Install-UpgradePackage([string]$ProjectRoot) {
    Expand-Archive -LiteralPath $upgradeZip -DestinationPath $ProjectRoot -Force
    if (-not (Test-Path (Join-Path $ProjectRoot '.code-harness-upgrade/bin/codea-dcep-tools.exe') -PathType Leaf)) { throw 'packaged upgrade Runtime missing' }
}

function Invoke-Upgrade([string]$ProjectRoot) {
    $previousErrorActionPreference = $ErrorActionPreference
    Push-Location $ProjectRoot
    try {
        $ErrorActionPreference = 'Continue'
        $lines = @(& '.\.code-harness-upgrade\bin\codea-dcep-tools.exe' upgrade 2>&1)
        $exit = $LASTEXITCODE
        return [pscustomobject]@{ ExitCode=$exit; Text=($lines | Out-String) }
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
        Pop-Location
        $global:LASTEXITCODE = 0
    }
}

function Assert-NoTransactionLeaks([string]$ProjectRoot) {
    $leaks = @(Get-ChildItem -LiteralPath $ProjectRoot -Force | Where-Object { $_.Name -like '.code-harness-backup-*' -or $_.Name -like '.code-harness-stage-*' })
    if ($leaks.Count -ne 0) { throw "transaction leaks remain: $($leaks.Name -join ',')" }
}

$expectedHead = Get-OfficialHead
Assert-UpgradePackage $expectedHead
$successRoot = Join-Path $env:RUNNER_TEMP ('task164-task2-upgrade-success-' + [guid]::NewGuid().ToString('N'))
$rollbackRoot = Join-Path $env:RUNNER_TEMP ('task164-task2-upgrade-rollback-' + [guid]::NewGuid().ToString('N'))
$deniedDirectory = $null
$originalAcl = $null
$watcher = $null
$createdSubscription = $null
$renamedSubscription = $null
try {
    Expand-Exact163Install $successRoot
    Assert-Exact163Install $successRoot
    Initialize-ProjectState $successRoot
    $successUnknownFrameworkHash = Get-FileSHA256 (Join-Path $successRoot '.code-harness/user-framework-file.txt')
    $successUnknownHostHash = Get-FileSHA256 (Join-Path $successRoot '.opencode/user-settings.json')
    Install-UpgradePackage $successRoot
    $success = Invoke-Upgrade $successRoot
    if ($success.ExitCode -ne 0 -or $success.Text -notmatch '"status"\s*:\s*"UPGRADED"') { throw "packaged 1.6.3 -> 1.6.4 upgrade failed: $($success.Text)" }
    if ((Get-Content -LiteralPath (Join-Path $successRoot '.code-harness/VERSION') -Raw).Trim() -ne '1.6.4') { throw 'upgrade did not install VERSION=1.6.4' }
    foreach ($rel in @('.opencode/agents/reviewer.md','.opencode/commands/harness-review-reviewer.md','.opencode/tools/codea-reviewer-submit.ts')) {
        if (-not (Test-Path (Join-Path $successRoot $rel) -PathType Leaf)) { throw "Runtime transaction did not install Reviewer Host file: $rel" }
    }
    if (-not (Test-Path (Join-Path $successRoot '.code-harness/user-framework-file.txt') -PathType Leaf) -or
        -not (Test-Path (Join-Path $successRoot '.opencode/user-settings.json') -PathType Leaf)) { throw 'successful transaction did not retain unknown user files' }
    if ((Get-FileSHA256 (Join-Path $successRoot '.code-harness/user-framework-file.txt')) -ne $successUnknownFrameworkHash -or
        (Get-FileSHA256 (Join-Path $successRoot '.opencode/user-settings.json')) -ne $successUnknownHostHash) { throw 'successful transaction changed unknown user files' }
    if (Test-Path (Join-Path $successRoot '.code-harness-upgrade')) { throw 'successful Runtime transaction did not consume upgrade source' }
    Assert-NoTransactionLeaks $successRoot
    Push-Location $successRoot
    try {
        $agents = (& opencode agent list 2>&1 | Out-String)
        $agentExit = $LASTEXITCODE
    } finally { Pop-Location; $global:LASTEXITCODE = 0 }
    if ($agentExit -ne 0 -or $agents -notmatch '(?m)^reviewer\b') { throw "installed Reviewer is not resolvable by actual OpenCode host:`n$agents" }
    Write-Output 'TASK164_TASK2_OPENCODE_AGENT_LIST_EVIDENCE'
    Write-Output ($agents.Trim())
    Write-Output "TASK164_TASK2_PACKAGED_163_TO_164_REVIEWER_RESOLVABLE PASS artifactId=$artifactId buildCommit=$exact163BuildCommit targetHead=$expectedHead"

    Expand-Exact163Install $rollbackRoot
    Assert-Exact163Install $rollbackRoot
    Initialize-ProjectState $rollbackRoot
    Install-UpgradePackage $rollbackRoot
    $deniedDirectory = Join-Path $rollbackRoot '.opencode/tools'
    New-Item -ItemType Directory -Force -Path $deniedDirectory | Out-Null
    [IO.File]::WriteAllText((Join-Path $deniedDirectory 'user-tool.txt'), "unknown-user-tool`r`n", $utf8)
    $frameworkBefore = Get-TreeSnapshot (Join-Path $rollbackRoot '.code-harness')
    $hostBefore = Get-TreeSnapshot (Join-Path $rollbackRoot '.opencode')
    $frameworkBeforeHash = Get-SnapshotSHA256 $frameworkBefore
    $hostBeforeHash = Get-SnapshotSHA256 $hostBefore

    # Deny file creation only in the third Host destination directory. Runtime
    # can preflight all three absent targets, then applies agent + command before
    # the submission-tool replacement fails. Framework and partial Host writes
    # must be restored by the packaged Runtime transaction.
    $originalAcl = Get-Acl -LiteralPath $deniedDirectory
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent().User
    $denyRights = [Security.AccessControl.FileSystemRights]::CreateFiles -bor [Security.AccessControl.FileSystemRights]::WriteData -bor [Security.AccessControl.FileSystemRights]::AppendData
    $denyRule = [Security.AccessControl.FileSystemAccessRule]::new($identity, $denyRights, [Security.AccessControl.InheritanceFlags]::None, [Security.AccessControl.PropagationFlags]::None, [Security.AccessControl.AccessControlType]::Deny)
    $blockedAcl = Get-Acl -LiteralPath $deniedDirectory
    $blockedAcl.AddAccessRule($denyRule)
    Set-Acl -LiteralPath $deniedDirectory -AclObject $blockedAcl

    # Capture OS filesystem events so the assertion proves the first two final
    # Host paths became observable before Runtime reached the denied third path.
    $watcher = [IO.FileSystemWatcher]::new((Join-Path $rollbackRoot '.opencode'))
    $watcher.IncludeSubdirectories = $true
    $watcher.EnableRaisingEvents = $true
    $eventPrefix = 'task164-host-' + [guid]::NewGuid().ToString('N')
    $createdSubscription = Register-ObjectEvent -InputObject $watcher -EventName Created -SourceIdentifier ($eventPrefix + '-created')
    $renamedSubscription = Register-ObjectEvent -InputObject $watcher -EventName Renamed -SourceIdentifier ($eventPrefix + '-renamed')
    $failed = Invoke-Upgrade $rollbackRoot
    Start-Sleep -Milliseconds 200
    $hostEvents = @(
        @(Get-Event -SourceIdentifier ($eventPrefix + '-created') -ErrorAction SilentlyContinue)
        @(Get-Event -SourceIdentifier ($eventPrefix + '-renamed') -ErrorAction SilentlyContinue)
    ) | ForEach-Object { $_.SourceEventArgs.FullPath }
    Unregister-Event -SourceIdentifier ($eventPrefix + '-created') -ErrorAction SilentlyContinue
    Unregister-Event -SourceIdentifier ($eventPrefix + '-renamed') -ErrorAction SilentlyContinue
    $createdSubscription = $null
    $renamedSubscription = $null
    $watcher.Dispose()
    $watcher = $null
    Set-Acl -LiteralPath $deniedDirectory -AclObject $originalAcl
    $originalAcl = $null
    if ($failed.ExitCode -eq 0 -or $failed.Text -notmatch '"status"\s*:\s*"UPGRADE_FAILED"' -or $failed.Text -notmatch '"rollbackPerformed"\s*:\s*true') {
        throw "Host partial-apply failure did not report successful rollback: $($failed.Text)"
    }
    if ($failed.Text -notmatch [regex]::Escape('.opencode/tools/codea-reviewer-submit.ts')) {
        throw "failure was not injected at the third Host apply target: $($failed.Text)"
    }
    foreach ($appliedRel in @('.opencode/agents/reviewer.md','.opencode/commands/harness-review-reviewer.md')) {
        $appliedPath = [IO.Path]::GetFullPath((Join-Path $rollbackRoot $appliedRel))
        if (-not @($hostEvents | Where-Object { [IO.Path]::GetFullPath($_) -eq $appliedPath }).Count) {
            throw "Host apply was not observed before rollback for $appliedRel; events=$($hostEvents -join ',')"
        }
    }
    Assert-SnapshotEqual 'framework rollback' $frameworkBefore (Get-TreeSnapshot (Join-Path $rollbackRoot '.code-harness'))
    Assert-SnapshotEqual 'Reviewer Host rollback' $hostBefore (Get-TreeSnapshot (Join-Path $rollbackRoot '.opencode'))
    if (-not (Test-Path (Join-Path $rollbackRoot '.code-harness/user-framework-file.txt') -PathType Leaf) -or
        -not (Test-Path (Join-Path $rollbackRoot '.opencode/user-settings.json') -PathType Leaf)) { throw 'rollback lost unknown user files' }
    if (-not (Test-Path (Join-Path $rollbackRoot '.code-harness-upgrade') -PathType Container)) { throw 'failed Runtime transaction consumed upgrade source' }
    Assert-NoTransactionLeaks $rollbackRoot
    Write-Output 'TASK164_TASK2_HOST_PARTIAL_APPLY_RUNTIME_EVIDENCE'
    Write-Output ($failed.Text.Trim())
    Write-Output "TASK164_TASK2_HOST_PARTIAL_APPLY_ROLLBACK PASS frameworkTreeSha256=$frameworkBeforeHash hostTreeSha256=$hostBeforeHash"
    Write-Output 'gate_task2_upgrade_e2e PASS'
} finally {
    if ($createdSubscription) { Unregister-Event -SourceIdentifier $createdSubscription.Name -ErrorAction SilentlyContinue }
    if ($renamedSubscription) { Unregister-Event -SourceIdentifier $renamedSubscription.Name -ErrorAction SilentlyContinue }
    if ($watcher) { $watcher.Dispose() }
    if ($originalAcl -and $deniedDirectory -and (Test-Path $deniedDirectory -PathType Container)) {
        Set-Acl -LiteralPath $deniedDirectory -AclObject $originalAcl -ErrorAction SilentlyContinue
    }
    Remove-Item -LiteralPath $successRoot -Recurse -Force -ErrorAction SilentlyContinue
    Remove-Item -LiteralPath $rollbackRoot -Recurse -Force -ErrorAction SilentlyContinue
}
