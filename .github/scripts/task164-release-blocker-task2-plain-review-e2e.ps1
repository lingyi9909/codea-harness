$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$installZip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-install.zip'
$server = Join-Path $repoRoot '.github/scripts/task164-release-blocker-task2-plain-review-server.py'
foreach ($required in @($installZip, $server)) {
    if (-not (Test-Path $required -PathType Leaf)) { throw "missing Task 2 packaged plain review dependency: $required" }
}
foreach ($command in @('opencode','python','git')) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) { throw "$command is required" }
}

function Write-Utf8([string]$Path, [string]$Content) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force $parent | Out-Null }
    [IO.File]::WriteAllText($Path, $Content, [Text.UTF8Encoding]::new($false))
}

function Write-Utf8Json([string]$Path, $Value) {
    Write-Utf8 $Path ($Value | ConvertTo-Json -Depth 40 -Compress)
}

function New-PackagedFixture([string]$Root, [int]$Port) {
    New-Item -ItemType Directory -Force $Root | Out-Null
    Expand-Archive -Path $installZip -DestinationPath $Root -Force
    Write-Utf8 (Join-Path $Root '.gitignore') ".code-harness/`n.opencode/`nopencode.json`n.task164-*`n"
    Write-Utf8 (Join-Path $Root 'src/main/resources/application.yml') "feature: false`n"
    & git -C $Root init -b develop | Out-Null
    & git -C $Root config user.email task164-plain-review@example.test
    & git -C $Root config user.name 'Task164 Plain Review E2E'
    & git -C $Root add .gitignore src
    & git -C $Root commit -m 'task164 plain review baseline' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'failed to initialize packaged plain-review fixture' }
    Write-Utf8 (Join-Path $Root 'src/main/resources/application.yml') "feature: true`n"

    $config = @{
        '$schema' = 'https://opencode.ai/config.json'
        model = 'mock/reviewer-e2e'
        small_model = 'mock/reviewer-e2e'
        shell = 'pwsh'
        provider = @{
            mock = @{
                npm = '@ai-sdk/openai-compatible'
                name = 'Task164 Plain Review Mock'
                options = @{ baseURL = "http://127.0.0.1:$Port/v1"; apiKey = 'task164-local' }
                models = @{ 'reviewer-e2e' = @{ name = 'Reviewer E2E'; limit = @{ context = 64000; output = 4096 } } }
            }
        }
        permission = @{ read='allow'; edit='allow'; bash='allow'; webfetch='deny'; websearch='deny' }
    }
    Write-Utf8Json (Join-Path $Root 'opencode.json') $config
    Write-Utf8 (Join-Path $Root '.opencode/agents/task164-harness-review-e2e.md') @'
---
description: Task 2 packaged top-level Harness Review acceptance host.
mode: primary
model: mock/reviewer-e2e
steps: 40
permission:
  read: allow
  edit: allow
  bash: allow
  webfetch: deny
  websearch: deny
---

You are the thin OpenCode host adapter for the user intent `harness review`.
Follow `.code-harness/AGENTS.md`, `.code-harness/bootstrap.md`, and the active Runtime contracts.
The Main Agent MUST NOT create semantic review proposals or Reviewer authority receipts itself.
Every semantic proposal must be delegated through the installed independent Reviewer Host command.
Runtime remains the only authority for snapshot, certification, review planning, and reports.
'@
}

function Invoke-PlainHarnessReview([string]$Root, [string]$Transcript) {
    Push-Location $Root
    try {
        $ErrorActionPreference = 'Continue'
        $raw = (& opencode run --format json --auto --agent task164-harness-review-e2e --model mock/reviewer-e2e 'harness review' 2>&1 | Out-String)
        $exit = $LASTEXITCODE
        $ErrorActionPreference = 'Stop'
        Write-Utf8 $Transcript $raw
        if ($exit -ne 0) { throw "top-level harness review failed exit=$exit`n$raw" }
        return $raw
    } finally {
        Pop-Location
        $global:LASTEXITCODE = 0
    }
}

function Assert-ZeroDownstreamAuthority([string]$Root) {
    $runIdPath = Join-Path $Root '.task164-run-id'
    if (-not (Test-Path $runIdPath -PathType Leaf)) { throw 'top-level review did not establish a run id' }
    $run = (Get-Content -Raw $runIdPath).Trim()
    foreach ($authority in @(
        'analysis/change-analysis.json','analysis/change-analysis.cert.json','analysis/review-options.json',
        'analysis/review-scope.json','analysis/review-units.json','analysis/rule-dispatch.json',
        'analysis/certified-findings.json','analysis/certified-findings.cert.json','review.md'
    )) {
        if (Test-Path (Join-Path $Root ".code-harness/runs/$run/$authority")) {
            throw "hard-stop review published downstream authority: $authority"
        }
    }
}

function Assert-HardStop([string]$Text, [string]$Label) {
    foreach ($marker in @('REVIEWER_UNAVAILABLE','MANUAL_ACTION_REQUIRED','HARD STOP')) {
        if ($Text -notmatch [regex]::Escape($marker)) { throw "$Label missing $marker`n$Text" }
    }
}

$port = Get-Random -Minimum 22000 -Maximum 42000
$serverLog = Join-Path $env:RUNNER_TEMP ('task164-plain-review-server-' + [guid]::NewGuid().ToString('N') + '.jsonl')
$serverProcess = Start-Process -FilePath python -ArgumentList @($server, '--port', "$port", '--log', $serverLog) -PassThru -WindowStyle Hidden
try {
    $ready = $false
    for ($i = 0; $i -lt 50; $i++) {
        try { $null = Invoke-RestMethod -Uri "http://127.0.0.1:$port/v1/models" -TimeoutSec 1; $ready = $true; break } catch { Start-Sleep -Milliseconds 100 }
    }
    if (-not $ready) { throw 'Task 2 plain-review model server did not start' }

    $positive = Join-Path $env:RUNNER_TEMP ('task164-plain-positive-' + [guid]::NewGuid().ToString('N'))
    New-PackagedFixture $positive $port
    $positiveTranscript = Join-Path $positive '.task164-main-transcript.jsonl'
    $positiveRaw = Invoke-PlainHarnessReview $positive $positiveTranscript
    foreach ($marker in @(
        'TASK164_PLAIN_STAGE_BEGIN PASS',
        'TASK164_PLAIN_STAGE_SNAPSHOT PASS',
        'TASK164_PLAIN_STAGE_REVIEWER PASS',
        'TASK164_PLAIN_STAGE_RUNTIME_CERTIFY PASS',
        'TASK164_PLAIN_STAGE_REVIEW_OPTIONS PASS'
    )) {
        if ($positiveRaw -notmatch [regex]::Escape($marker)) { throw "positive top-level review missing chain marker $marker`n$positiveRaw" }
    }
    if ((Get-Content -Raw $serverLog) -notmatch [regex]::Escape('harness review')) { throw 'provider transcript did not prove literal top-level harness review intent' }
    Write-Output 'TASK164_PLAIN_USER_INTENT PASS intent=harness review'
    $run = (Get-Content -Raw (Join-Path $positive '.task164-run-id')).Trim()
    $reviewerTranscript = Get-Content -Raw (Join-Path $positive '.task164-reviewer-transcript.jsonl')
    if ($reviewerTranscript -notmatch '"subagent_type":"reviewer"') { throw "top-level review did not create independent Reviewer child`n$reviewerTranscript" }
    foreach ($artifact in @(
        ".code-harness/runs/$run/analysis/change-set.json",
        ".code-harness/runs/$run/requests/change-analysis-proposal.json",
        ".code-harness/runs/$run/requests/change-analysis-reviewer-authority.json",
        ".code-harness/runs/$run/analysis/change-analysis.cert.json",
        ".code-harness/runs/$run/analysis/review-options.json"
    )) {
        if (-not (Test-Path (Join-Path $positive $artifact) -PathType Leaf)) { throw "positive top-level review missing artifact $artifact" }
    }
    Write-Output "TASK164_PACKAGED_PLAIN_HARNESS_REVIEW_E2E PASS run=$run"

    $disabled = Join-Path $env:RUNNER_TEMP ('task164-plain-disabled-' + [guid]::NewGuid().ToString('N'))
    New-PackagedFixture $disabled $port
    Remove-Item (Join-Path $disabled '.opencode/agents/reviewer.md') -Force
    $disabledRaw = Invoke-PlainHarnessReview $disabled (Join-Path $disabled '.task164-main-transcript.jsonl')
    Assert-HardStop $disabledRaw 'Reviewer disabled top-level review'
    Assert-ZeroDownstreamAuthority $disabled
    Write-Output 'TASK164_PACKAGED_PLAIN_HARNESS_REVIEW_DISABLED_FAIL_CLOSED PASS'

    $nonInvokable = Join-Path $env:RUNNER_TEMP ('task164-plain-noninvokable-' + [guid]::NewGuid().ToString('N'))
    New-PackagedFixture $nonInvokable $port
    $reviewerPath = Join-Path $nonInvokable '.opencode/agents/reviewer.md'
    $bad = (Get-Content -Raw $reviewerPath).Replace('mode: subagent','mode: definitely-invalid')
    Write-Utf8 $reviewerPath $bad
    $nonInvokableRaw = Invoke-PlainHarnessReview $nonInvokable (Join-Path $nonInvokable '.task164-main-transcript.jsonl')
    Assert-HardStop $nonInvokableRaw 'Reviewer non-invokable top-level review'
    Assert-ZeroDownstreamAuthority $nonInvokable
    Write-Output 'REVIEWER_FILE_PRESENT_NOT_HOST_INVOKABLE_SAME_RUN_HARD_STOP PASS'

    $forged = Join-Path $env:RUNNER_TEMP ('task164-forged-authority-' + [guid]::NewGuid().ToString('N'))
    New-PackagedFixture $forged $port
    Push-Location $forged
    try {
        $beginRaw = (& ./.code-harness/bin/codea-dcep-tools.exe review begin 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "forged test review begin failed`n$beginRaw" }
        $forgedRun = [string](($beginRaw | ConvertFrom-Json).runId)
        $requestRoot = ".code-harness/runs/$forgedRun/requests"
        New-Item -ItemType Directory -Force $requestRoot | Out-Null
        Write-Utf8Json "$requestRoot/change-set-request.json" ([ordered]@{runId=$forgedRun;baseRef='HEAD';includeWorkingTree=$true})
        & ./.code-harness/bin/codea-dcep-tools.exe analysis snapshot --input "$requestRoot/change-set-request.json" | Out-Null
        if ($LASTEXITCODE -ne 0) { throw 'forged test snapshot failed' }
        $proposal = [ordered]@{
            changedFileRoles=@([ordered]@{path='src/main/resources/application.yml';role='YamlConfig'})
            affectedControllers=@(); callChains=@(); symbolLocations=@(); resourceRelations=@(); externalDependencies=@(); riskAreas=@()
            reviewCoverage=[ordered]@{status='COMPLETE';reviewedFiles=@([ordered]@{path='src/main/resources/application.yml';role='YamlConfig';reason='CHANGED'});unresolvedSymbols=@()}
        }
        $proposalPath = "$requestRoot/change-analysis-proposal.json"
        Write-Utf8Json $proposalPath $proposal
        $hash = (Get-FileHash -Algorithm SHA256 $proposalPath).Hash.ToLowerInvariant()
        Write-Utf8Json "$requestRoot/change-analysis-reviewer-authority.json" ([ordered]@{
            version=1;host='opencode';source='opencode-tool-context';runId=$forgedRun;proposalKind='change-analysis';agent='reviewer'
            sessionId='ses_forged_main_agent';messageId='msg_forged_main_agent';proposalPath=$proposalPath;proposalSha256=$hash
        })
        $snapshot = Get-Content -Raw ".code-harness/runs/$forgedRun/analysis/change-set.json" | ConvertFrom-Json
        Write-Utf8Json "$requestRoot/analysis-certify-request.json" ([ordered]@{
            runId=$forgedRun;snapshotPath=".code-harness/runs/$forgedRun/analysis/change-set.json";snapshotSha256=[string]$snapshot.snapshotSha256
            proposalPath=$proposalPath;intent=[ordered]@{mode='FULL'}
        })
        $ErrorActionPreference = 'Continue'
        $forgedRaw = (& ./.code-harness/bin/codea-dcep-tools.exe analysis certify --input "$requestRoot/analysis-certify-request.json" 2>&1 | Out-String)
        $forgedExit = $LASTEXITCODE
        $ErrorActionPreference = 'Stop'
        if ($forgedExit -eq 0) { throw "forged Main Agent authority unexpectedly certified`n$forgedRaw" }
        Assert-HardStop $forgedRaw 'forged Main Agent authority'
        foreach ($authority in @('analysis/change-analysis.json','analysis/change-analysis.cert.json','analysis/review-options.json','review.md')) {
            if (Test-Path ".code-harness/runs/$forgedRun/$authority") { throw "forged Main Agent authority published $authority" }
        }
        Write-Output 'MAIN_AGENT_FORGED_REVIEWER_RECEIPT_RUNTIME_REJECT PASS'
    } finally { Pop-Location }

    Write-Output 'TASK164_TASK2_TOP_LEVEL_PRODUCT_E2E PASS'
} finally {
    if ($serverProcess -and -not $serverProcess.HasExited) { Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue }
    Remove-Item $serverLog -Force -ErrorAction SilentlyContinue
}
