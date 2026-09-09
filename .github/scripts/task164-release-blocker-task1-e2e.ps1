$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$artifactId = 10004022218
$exact163BuildCommit = '2278ad49d4534b57f3144466ffe0e65ddca7f189'
$exact163SchemaNormalizedSha256 = '0a192ce95adb9ff69eeb5c89cabfc8557e17437faa00e094bba7ba2bd49febf2'
$utf8 = [Text.UTF8Encoding]::new($false)

function Get-TextSHA256([string]$Text) {
    $sha = [Security.Cryptography.SHA256]::Create()
    try {
        return ([BitConverter]::ToString($sha.ComputeHash([Text.Encoding]::UTF8.GetBytes($Text)))).Replace('-', '').ToLowerInvariant()
    } finally {
        $sha.Dispose()
    }
}

function Get-FileSHA256([string]$Path) {
    return (Get-FileHash -Algorithm SHA256 $Path).Hash.ToLowerInvariant()
}

function Expand-Exact163Install([string]$Destination) {
    $outer = Join-Path $env:RUNNER_TEMP ('task164-163-artifact-' + [guid]::NewGuid().ToString('N') + '.zip')
    $artifactDir = Join-Path $env:RUNNER_TEMP ('task164-163-artifact-' + [guid]::NewGuid().ToString('N'))
    try {
        $headers = @{
            Authorization = "Bearer $env:GH_TOKEN"
            Accept = 'application/vnd.github+json'
            'X-GitHub-Api-Version' = '2022-11-28'
        }
        Invoke-WebRequest -Uri "https://api.github.com/repos/lingyi9909/codea-harness/actions/artifacts/$artifactId/zip" -Headers $headers -OutFile $outer
        Expand-Archive $outer $artifactDir -Force
        $releaseZips = @(Get-ChildItem $artifactDir -File -Filter 'codea-harness-1.6.3-windows-x64-install.zip')
        if ($releaseZips.Count -ne 1) {
            throw "expected one exact 1.6.3 install ZIP, found $($releaseZips.Count)"
        }
        New-Item -ItemType Directory -Force $Destination | Out-Null
        Expand-Archive $releaseZips[0].FullName $Destination -Force
    } finally {
        Remove-Item $outer -Force -ErrorAction SilentlyContinue
        Remove-Item $artifactDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

function Assert-Exact163Install([string]$ProjectRoot) {
    $target = Join-Path $ProjectRoot '.code-harness'
    if ((Get-Content (Join-Path $target 'VERSION') -Raw).Trim() -ne '1.6.3') {
        throw 'official artifact VERSION is not 1.6.3'
    }
    $manifest = Get-Content (Join-Path $target 'RELEASE-MANIFEST.json') -Raw | ConvertFrom-Json
    if ([string]$manifest.version -ne '1.6.3' -or [string]$manifest.buildCommit -ne $exact163BuildCommit) {
        throw "unexpected exact 1.6.3 manifest version/buildCommit: $($manifest.version) $($manifest.buildCommit)"
    }
    if ([string]$manifest.platform -ne 'windows' -or [string]$manifest.arch -ne 'x64') {
        throw 'official 1.6.3 artifact platform mismatch'
    }
}

function Assert-SchemaParity([string]$ProjectRoot) {
    $oldSchema = Get-Content (Join-Path $ProjectRoot '.code-harness/contracts/harness-config.schema.json') -Raw
    $newSchema = Get-Content (Join-Path $repoRoot '.code-harness/contracts/harness-config.schema.json') -Raw
    $oldNormalized = $oldSchema.Replace("`r`n", "`n")
    $newNormalized = $newSchema.Replace("`r`n", "`n")
    $oldHash = Get-TextSHA256 $oldNormalized
    $newHash = Get-TextSHA256 $newNormalized
    if ($oldHash -ne $exact163SchemaNormalizedSha256 -or $newHash -ne $exact163SchemaNormalizedSha256) {
        throw "exact schema fingerprint mismatch old=$oldHash new=$newHash"
    }
    if ($oldNormalized -cne $newNormalized) {
        throw 'exact 1.6.3 and 1.6.4 harness schemas differ'
    }
    Write-Output "CONFIG_SCHEMA_163_164_BACKWARD_COMPATIBLE PASS normalizedSha256=$newHash"
}

function Initialize-RealProjectState([string]$ProjectRoot) {
    $target = Join-Path $ProjectRoot '.code-harness'
    $template = Get-Content (Join-Path $target 'harness.template.yaml') -Raw
    $config = $template.Replace('module: ""', 'module: "payments-user-owned"')
    $config = $config.Replace('baseRef: ""', 'baseRef: origin/develop')
    $config = $config.Replace('timeoutSeconds: 600', 'timeoutSeconds: 777')
    if (-not $config.Contains('module: "payments-user-owned"') -or -not $config.Contains('baseRef: origin/develop') -or -not $config.Contains('timeoutSeconds: 777')) {
        throw 'failed to materialize real 1.6.3 user config sentinels'
    }
    [IO.File]::WriteAllText((Join-Path $target 'harness.yaml'), $config, $utf8)
    [IO.File]::WriteAllText((Join-Path $target 'project.md'), "user-project-state`r`n", $utf8)
    New-Item -ItemType Directory -Force (Join-Path $target 'chains'),(Join-Path $target 'runs/user-run'),(Join-Path $target 'skills/user-owned') | Out-Null
    [IO.File]::WriteAllText((Join-Path $target 'chains/user-chain.yaml'), "version: 1`r`nid: user-chain`r`n", $utf8)
    [IO.File]::WriteAllText((Join-Path $target 'runs/user-run/evidence.txt'), "user-run-state`r`n", $utf8)
    [IO.File]::WriteAllText((Join-Path $target 'skills/user-owned/keep.txt'), "unknown-user-file`r`n", $utf8)
}

function Get-ProjectSentinelHashes([string]$ProjectRoot) {
    $target = Join-Path $ProjectRoot '.code-harness'
    $hashes = [ordered]@{}
    foreach ($rel in @(
        'harness.yaml',
        'project.md',
        'chains/user-chain.yaml',
        'runs/user-run/evidence.txt',
        'skills/user-owned/keep.txt'
    )) {
        $hashes[$rel] = Get-FileSHA256 (Join-Path $target $rel)
    }
    return $hashes
}

function Assert-ProjectSentinelHashes([string]$ProjectRoot, $Expected) {
    $target = Join-Path $ProjectRoot '.code-harness'
    foreach ($entry in $Expected.GetEnumerator()) {
        $path = Join-Path $target $entry.Key
        if (-not (Test-Path $path -PathType Leaf)) {
            throw "preserved user file missing: $($entry.Key)"
        }
        $actual = Get-FileSHA256 $path
        if ($actual -ne $entry.Value) {
            throw "preserved user file changed: $($entry.Key) actual=$actual expected=$($entry.Value)"
        }
    }
}

function Install-CandidateUpgrade([string]$ProjectRoot) {
    $candidate = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-upgrade.zip'
    if (-not (Test-Path $candidate -PathType Leaf)) {
        throw "missing candidate 1.6.4 upgrade ZIP: $candidate"
    }
    Expand-Archive $candidate $ProjectRoot -Force
    if (-not (Test-Path (Join-Path $ProjectRoot '.code-harness-upgrade/bin/codea-dcep-tools.exe') -PathType Leaf)) {
        throw 'candidate upgrade Runtime missing'
    }
}

function Invoke-CandidateUpgrade([string]$ProjectRoot) {
    Push-Location $ProjectRoot
    try {
        $lines = @(& '.\.code-harness-upgrade\bin\codea-dcep-tools.exe' upgrade 2>&1)
        $exit = $LASTEXITCODE
        $text = $lines | Out-String
        $lines | ForEach-Object { Write-Host $_ }
        return [pscustomobject]@{ ExitCode = $exit; Text = $text }
    } finally {
        Pop-Location
        $global:LASTEXITCODE = 0
    }
}

if ([string]::IsNullOrWhiteSpace($env:GH_TOKEN)) {
    throw 'GH_TOKEN is required to retrieve exact packaged 1.6.3 artifact'
}

$successRoot = Join-Path $env:RUNNER_TEMP ('task164-config-success-' + [guid]::NewGuid().ToString('N'))
$failureRoot = Join-Path $env:RUNNER_TEMP ('task164-config-failure-' + [guid]::NewGuid().ToString('N'))
try {
    Expand-Exact163Install $successRoot
    Assert-Exact163Install $successRoot
    Assert-SchemaParity $successRoot
    Initialize-RealProjectState $successRoot
    $before = Get-ProjectSentinelHashes $successRoot

    Install-CandidateUpgrade $successRoot
    $first = Invoke-CandidateUpgrade $successRoot
    if ($first.ExitCode -ne 0 -or $first.Text -notmatch '"status"\s*:\s*"UPGRADED"') {
        throw "real packaged 1.6.3 -> 1.6.4 upgrade failed: $($first.Text)"
    }
    if ($first.Text -notmatch [regex]::Escape('config-1.6.3-to-1.6.4')) {
        throw "registered 1.6.3 -> 1.6.4 migration missing from product output: $($first.Text)"
    }
    if ((Get-Content (Join-Path $successRoot '.code-harness/VERSION') -Raw).Trim() -ne '1.6.4') {
        throw 'successful product upgrade did not install VERSION=1.6.4'
    }
    Assert-ProjectSentinelHashes $successRoot $before
    if (Test-Path (Join-Path $successRoot '.code-harness-upgrade')) {
        throw 'successful upgrade did not consume candidate source tree'
    }
    Write-Output 'CONFIG_MIGRATION_163_TO_164_REGISTERED PASS'
    Write-Output 'CONFIG_MIGRATION_BEFORE_TARGET_SCHEMA_VALIDATION PASS'
    Write-Output 'CONFIG_MIGRATION_TARGET_SCHEMA_VALID PASS'
    Write-Output 'CONFIG_USER_VALUES_PRESERVED PASS'
    Write-Output 'CONFIG_UNKNOWN_USER_FILES_PRESERVED PASS'

    Install-CandidateUpgrade $successRoot
    $second = Invoke-CandidateUpgrade $successRoot
    if ($second.ExitCode -ne 0 -or $second.Text -notmatch '"status"\s*:\s*"ALREADY_UP_TO_DATE"') {
        throw "second exact candidate upgrade was not idempotent: $($second.Text)"
    }
    Assert-ProjectSentinelHashes $successRoot $before
    Remove-Item (Join-Path $successRoot '.code-harness-upgrade') -Recurse -Force -ErrorAction SilentlyContinue
    Write-Output 'CONFIG_MIGRATION_IDEMPOTENT PASS'

    Expand-Exact163Install $failureRoot
    Assert-Exact163Install $failureRoot
    Initialize-RealProjectState $failureRoot
    $badConfigPath = Join-Path $failureRoot '.code-harness/harness.yaml'
    $badConfig = (Get-Content $badConfigPath -Raw).Replace('version: 2', 'version: 3')
    [IO.File]::WriteAllText($badConfigPath, $badConfig, $utf8)
    $failureBefore = Get-ProjectSentinelHashes $failureRoot
    $versionBefore = Get-FileSHA256 (Join-Path $failureRoot '.code-harness/VERSION')
    $manifestBefore = Get-FileSHA256 (Join-Path $failureRoot '.code-harness/RELEASE-MANIFEST.json')

    Install-CandidateUpgrade $failureRoot
    $failed = Invoke-CandidateUpgrade $failureRoot
    if ($failed.ExitCode -eq 0 -or $failed.Text -notmatch '"status"\s*:\s*"MANUAL_ACTION_REQUIRED"') {
        throw "unsupported config did not fail closed: $($failed.Text)"
    }
    if ($failed.Text -notmatch 'version must be integer 1 or 2') {
        throw "unsupported config failure reason was not migration authority: $($failed.Text)"
    }
    Assert-ProjectSentinelHashes $failureRoot $failureBefore
    if ((Get-FileSHA256 (Join-Path $failureRoot '.code-harness/VERSION')) -ne $versionBefore) {
        throw 'failed migration changed installed VERSION'
    }
    if ((Get-FileSHA256 (Join-Path $failureRoot '.code-harness/RELEASE-MANIFEST.json')) -ne $manifestBefore) {
        throw 'failed migration changed installed manifest'
    }
    if (-not (Test-Path (Join-Path $failureRoot '.code-harness-upgrade') -PathType Container)) {
        throw 'failed migration consumed source upgrade package'
    }
    $transactionLeaks = @(Get-ChildItem $failureRoot -Force | Where-Object { $_.Name -like '.code-harness-backup-*' -or $_.Name -like '.code-harness-stage-*' })
    if ($transactionLeaks.Count -ne 0) {
        throw "failed migration left transaction artifacts: $($transactionLeaks.Name -join ',')"
    }
    Write-Output 'CONFIG_UNSUPPORTED_MIGRATION_FAIL_CLOSED PASS'
    Write-Output "TASK164_CONFIG_PACKAGED_163_TO_164_E2E PASS artifactId=$artifactId buildCommit=$exact163BuildCommit"
} finally {
    Remove-Item $successRoot,$failureRoot -Recurse -Force -ErrorAction SilentlyContinue
}
