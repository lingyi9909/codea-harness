$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$runtimeSource = Join-Path $repoRoot '.code-harness/bin/codea-dcep-tools.exe'
$astGrepSource = Join-Path $repoRoot '.code-harness/bin/ast-grep.exe'
$modelServer = Join-Path $PSScriptRoot 'task162-review-reliability-task2/mock_openai_server.py'
$artifactRoot = Join-Path $repoRoot '.task2-e2e-artifacts'

foreach ($required in @($runtimeSource, $astGrepSource, $modelServer)) {
    if (!(Test-Path $required -PathType Leaf)) { throw "Task 2 required file missing: $required" }
}
if (-not (Get-Command opencode -ErrorAction SilentlyContinue)) { throw 'Task 2 requires pinned OpenCode CLI on PATH' }
if (-not (Get-Command python -ErrorAction SilentlyContinue)) { throw 'Task 2 requires Python on PATH' }
New-Item -ItemType Directory -Force $artifactRoot | Out-Null

function Write-Utf8NoBom([string]$Path, [string]$Content) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force $parent | Out-Null }
    [System.IO.File]::WriteAllText($Path, $Content, [System.Text.UTF8Encoding]::new($false))
}

function Invoke-Git([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments) {
    & git @Arguments | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "git $($Arguments -join ' ') failed with exit code $LASTEXITCODE" }
}

function Get-ReviewRunIds([string]$Fixture) {
    $runs = Join-Path $Fixture '.code-harness/runs'
    if (!(Test-Path $runs -PathType Container)) { return @() }
    return @(
        Get-ChildItem $runs -Directory |
            Where-Object { $_.Name -match '^review-[0-9a-f]{32}$' } |
            ForEach-Object { $_.Name } |
            Sort-Object
    )
}

function Assert-FormalRun([string]$Fixture, [string]$RunId, [string[]]$ExpectedPaths, [string]$ExpectedSource) {
    $runRoot = Join-Path $Fixture ".code-harness/runs/$RunId"
    $analysis = Join-Path $runRoot 'analysis'
    foreach ($name in @(
        'change-set.json',
        'change-analysis.json',
        'entrypoint-inventory.json',
        'change-analysis.cert.json',
        'review-options.json',
        'review-scope.json',
        'review-units.json',
        'rule-dispatch.json',
        'certified-findings.json',
        'certified-findings.cert.json'
    )) {
        $path = Join-Path $analysis $name
        if (!(Test-Path $path -PathType Leaf)) { throw "Task 2 run $RunId missing formal artifact: $path" }
    }
    $reportPath = Join-Path $runRoot 'review.md'
    if (!(Test-Path $reportPath -PathType Leaf)) { throw "Task 2 run $RunId missing review.md" }
    $report = Get-Content -Raw $reportPath
    if ($report -notmatch '评审结果' -or $report -notmatch '通过') { throw "Task 2 run $RunId report not PASSED:`n$report" }

    $snapshot = Get-Content -Raw (Join-Path $analysis 'change-set.json') | ConvertFrom-Json
    $paths = @($snapshot.files | ForEach-Object { [string]$_.path })
    if (($paths -join '|') -ne ($ExpectedPaths -join '|')) {
        throw "Task 2 run $RunId Snapshot paths mismatch. actual=$($paths -join ',') expected=$($ExpectedPaths -join ',')"
    }
    if ($ExpectedSource) {
        if (@($snapshot.files).Count -ne 1) { throw "Task 2 run $RunId expected one changed file" }
        $sources = @($snapshot.files[0].sources | ForEach-Object { [string]$_ })
        if ($sources -notcontains $ExpectedSource) {
            throw "Task 2 run $RunId missing expected source $ExpectedSource; actual=$($sources -join ',')"
        }
    }
    return $snapshot
}

function Invoke-SameSessionScenario([ValidateSet('working-tree','head')][string]$Scenario) {
    $fixture = Join-Path $env:RUNNER_TEMP ("task162-task2-$Scenario-" + [guid]::NewGuid().ToString('N'))
    $serverLog = Join-Path $artifactRoot ("$Scenario-model.jsonl")
    $firstTranscript = Join-Path $artifactRoot ("$Scenario-first-opencode.jsonl")
    $secondTranscript = Join-Path $artifactRoot ("$Scenario-second-opencode.jsonl")
    $serverProcess = $null

    $listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
    $listener.Start()
    $port = ([System.Net.IPEndPoint]$listener.LocalEndpoint).Port
    $listener.Stop()

    try {
        New-Item -ItemType Directory -Force $fixture | Out-Null
        Copy-Item (Join-Path $repoRoot '.code-harness') (Join-Path $fixture '.code-harness') -Recurse -Force
        Remove-Item (Join-Path $fixture '.code-harness/runs') -Recurse -Force -ErrorAction SilentlyContinue

        Write-Utf8NoBom (Join-Path $fixture 'pom.xml') @'
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.acme</groupId>
  <artifactId>task162-fresh-review-fixture</artifactId>
  <version>1.0.0</version>
</project>
'@
        Write-Utf8NoBom (Join-Path $fixture 'src/main/resources/application.yml') @'
feature:
  reliability: false
'@
        Write-Utf8NoBom (Join-Path $fixture '.code-harness/harness.yaml') @'
version: 2
project:
  type: maven
  root: .
  module: ""
review:
  baseRef: review-base
  includeWorkingTree: true
scope:
  sourceIncludes:
    - src/main/java/**/*.java
  testIncludes:
    - src/test/java/**/*.java
  mapperIncludes:
    - src/main/resources/**/*Mapper.xml
  configIncludes:
    - src/main/resources/**/*.yml
runs:
  directory: .code-harness/runs
'@

        $opencodeConfig = @{
            '$schema' = 'https://opencode.ai/config.json'
            model = 'task2-local/task2'
            small_model = 'task2-local/task2'
            shell = 'pwsh'
            provider = @{
                'task2-local' = @{
                    npm = '@ai-sdk/openai-compatible'
                    name = 'Task2 Local Stateful Deterministic'
                    options = @{
                        baseURL = "http://127.0.0.1:$port/v1"
                        apiKey = 'task2-local'
                    }
                    models = @{
                        task2 = @{
                            name = 'Task2 Stateful Deterministic'
                            limit = @{ context = 300000; output = 4096 }
                        }
                    }
                }
            }
            permission = @{
                read = 'allow'
                edit = 'allow'
                bash = 'allow'
                webfetch = 'deny'
                websearch = 'deny'
            }
        }
        Write-Utf8NoBom (Join-Path $fixture 'opencode.json') ($opencodeConfig | ConvertTo-Json -Depth 20)
        Write-Utf8NoBom (Join-Path $fixture '.opencode/agents/codea-harness-e2e.md') @'
---
description: Executes Codea Harness Fresh Review lifecycle through active repository contracts
mode: primary
model: task2-local/task2
steps: 100
permission:
  read: allow
  edit: allow
  bash: allow
  webfetch: deny
  websearch: deny
---

You are the thin Agent Host adapter for Codea Harness acceptance testing.
Every new top-level `harness review` is a fresh invocation. Read active contracts, call Runtime `review begin`, use only the returned fresh runId, then execute a fresh Runtime `analysis snapshot` and the normal formal Review authority chain.
Never reuse an earlier runId, Snapshot, ChangeAnalysis, zero-change conclusion, or review.md merely because they are present in this session.
'@

        Push-Location $fixture
        try {
            Invoke-Git init
            Invoke-Git config user.email 'task162-task2@example.test'
            Invoke-Git config user.name 'Task 162 Task 2 E2E'
            Invoke-Git config core.autocrlf false
            Invoke-Git add .
            Invoke-Git commit -m 'baseline fresh review fixture'
            Invoke-Git branch review-base

            $serverProcess = Start-Process -FilePath 'python' -ArgumentList @($modelServer, '--port', "$port", '--log', $serverLog, '--scenario', $Scenario) -PassThru -WindowStyle Hidden
            $healthy = $false
            for ($i = 0; $i -lt 40; $i++) {
                try {
                    $health = Invoke-RestMethod -Uri "http://127.0.0.1:$port/health" -TimeoutSec 2
                    if ($health.status -eq 'ok') { $healthy = $true; break }
                }
                catch { Start-Sleep -Milliseconds 250 }
            }
            if (-not $healthy) { throw "Task 2 $Scenario model server did not become healthy" }

            $plainUserIntent = 'harness review'
            $ErrorActionPreference = 'Continue'
            $raw1 = (& opencode run --format json --auto --agent codea-harness-e2e --model task2-local/task2 $plainUserIntent 2>&1 | Out-String)
            $exit1 = $LASTEXITCODE
            $ErrorActionPreference = 'Stop'
            Write-Utf8NoBom $firstTranscript $raw1
            if ($exit1 -ne 0) { throw "Task 2 $Scenario first harness review failed with exit ${exit1}:`n$raw1" }

            $runsAfterFirst = @(Get-ReviewRunIds $fixture)
            if ($runsAfterFirst.Count -ne 1) { throw "Task 2 $Scenario first invocation must create exactly one fresh Runtime run; actual=$($runsAfterFirst -join ',')" }
            $runA = [string]$runsAfterFirst[0]
            $snapshotA = Assert-FormalRun $fixture $runA @() ''
            $headA = [string]$snapshotA.headCommit
            $stateA = [string]$snapshotA.gitStateSha256

            if ($Scenario -eq 'working-tree') {
                Write-Utf8NoBom (Join-Path $fixture 'src/main/resources/application.yml') @'
feature:
  reliability: true
'@
                $status = @((git status --porcelain) | Where-Object { $_ -match 'src/main/resources/application.yml$' })
                if ($status.Count -ne 1) { throw "Task 2 working-tree mutation not present: $($status -join '; ')" }
            }
            else {
                Write-Utf8NoBom (Join-Path $fixture 'src/main/resources/application.yml') @'
feature:
  reliability: true
'@
                Invoke-Git add 'src/main/resources/application.yml'
                Invoke-Git commit -m 'change review relevant HEAD state'
            }

            $ErrorActionPreference = 'Continue'
            $raw2 = (& opencode run --continue --format json --auto --agent codea-harness-e2e --model task2-local/task2 $plainUserIntent 2>&1 | Out-String)
            $exit2 = $LASTEXITCODE
            $ErrorActionPreference = 'Stop'
            Write-Utf8NoBom $secondTranscript $raw2
            if ($exit2 -ne 0) { throw "Task 2 $Scenario second same-session harness review failed with exit ${exit2}:`n$raw2" }

            $runsAfterSecond = @(Get-ReviewRunIds $fixture)
            if ($runsAfterSecond.Count -ne 2) { throw "Task 2 $Scenario second invocation must create a second fresh Runtime run; actual=$($runsAfterSecond -join ',')" }
            $runB = [string](@($runsAfterSecond | Where-Object { $_ -ne $runA })[0])
            if (-not $runB -or $runA -eq $runB) { throw "Task 2 $Scenario did not produce a distinct runId. A=$runA B=$runB" }

            $expectedSource = if ($Scenario -eq 'working-tree') { 'UNSTAGED' } else { 'COMMITTED' }
            $snapshotB = Assert-FormalRun $fixture $runB @('src/main/resources/application.yml') $expectedSource
            $headB = [string]$snapshotB.headCommit
            $stateB = [string]$snapshotB.gitStateSha256

            if ($Scenario -eq 'working-tree') {
                if ($headA -ne $headB) { throw "Working-tree scenario unexpectedly changed HEAD. A=$headA B=$headB" }
                if ($stateA -eq $stateB) { throw 'Working-tree scenario did not produce a fresh Git state identity' }
            }
            else {
                if ($headA -eq $headB) { throw "HEAD scenario did not observe new commit. A=$headA B=$headB" }
            }

            if ($raw2 -match '代码已经变化|代码变化了|重新拉了新的代码') {
                throw 'Task 2 E2E must not rely on a user freshness hint'
            }
            if ($raw1 -match 'TASK2_E2E_ABORT' -or $raw2 -match 'TASK2_E2E_ABORT') {
                throw "Task 2 $Scenario model aborted authority chain"
            }

            $events = @()
            foreach ($line in @(Get-Content $serverLog)) {
                if ($line.Trim()) { $events += ($line | ConvertFrom-Json) }
            }
            $runEvents = @($events | Where-Object { $_.event -eq 'invocation_run' })
            if ($runEvents.Count -ne 2) { throw "Task 2 $Scenario model did not observe exactly two fresh invocation runIds; count=$($runEvents.Count)" }
            $modelA = @($runEvents | Where-Object { [int]$_.invocation -eq 1 })
            $modelB = @($runEvents | Where-Object { [int]$_.invocation -eq 2 })
            if ($modelA.Count -ne 1 -or [string]$modelA[0].runId -ne $runA) { throw "Task 2 $Scenario model run-A mismatch" }
            if ($modelB.Count -ne 1 -or [string]$modelB[0].runId -ne $runB) { throw "Task 2 $Scenario model run-B mismatch" }

            $sameSessionEvidence = @($events | Where-Object { $_.event -eq 'request' -and [int]$_.invocation -eq 2 -and [int]$_.reviewUserCount -ge 2 })
            if ($sameSessionEvidence.Count -eq 0) { throw "Task 2 $Scenario second request did not contain first invocation history; OpenCode session was not continued" }

            $beginCalls = @($events | Where-Object { $_.event -eq 'tool_call' -and [int]$_.stage -eq 1 -and ([string]$_.command) -match 'review begin' })
            if (@($beginCalls | Where-Object { [int]$_.invocation -eq 1 }).Count -eq 0 -or @($beginCalls | Where-Object { [int]$_.invocation -eq 2 }).Count -eq 0) {
                throw "Task 2 $Scenario did not call review begin in both invocations"
            }
            $snapshotCalls = @($events | Where-Object { $_.event -eq 'tool_call' -and [int]$_.stage -eq 3 -and ([string]$_.command) -match 'analysis snapshot' })
            if (@($snapshotCalls | Where-Object { [int]$_.invocation -eq 1 }).Count -eq 0 -or @($snapshotCalls | Where-Object { [int]$_.invocation -eq 2 }).Count -eq 0) {
                throw "Task 2 $Scenario did not execute fresh Runtime Snapshot in both invocations"
            }

            $scenarioUpper = $Scenario.ToUpperInvariant().Replace('-', '_')
            Write-Output "TASK162_REVIEW_RELIABILITY_TASK2_${scenarioUpper}_SAME_SESSION PASS"
            Write-Output "TASK162_REVIEW_RELIABILITY_TASK2_${scenarioUpper}_FRESH_RUN_IDS PASS runA=$runA runB=$runB"
            Write-Output "TASK162_REVIEW_RELIABILITY_TASK2_${scenarioUpper}_FRESH_SNAPSHOT PASS"
        }
        finally { Pop-Location }
    }
    finally {
        if ($null -ne $serverProcess -and -not $serverProcess.HasExited) { Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue }
        Remove-Item $fixture -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Invoke-SameSessionScenario -Scenario 'working-tree'
Invoke-SameSessionScenario -Scenario 'head'
Write-Output 'TASK162_REVIEW_RELIABILITY_TASK2_REAL_AGENT_E2E PASS'
