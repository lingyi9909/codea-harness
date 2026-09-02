$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$runtimeSource = Join-Path $repoRoot '.code-harness\bin\codea-dcep-tools.exe'
$astGrepSource = Join-Path $repoRoot '.code-harness\bin\ast-grep.exe'
$modelServer = Join-Path $PSScriptRoot 'task162-hotfix-task3\mock_openai_server.py'

foreach ($required in @($runtimeSource, $astGrepSource, $modelServer)) {
    if (!(Test-Path $required -PathType Leaf)) { throw "Task 3 required file missing: $required" }
}
if (-not (Get-Command opencode -ErrorAction SilentlyContinue)) { throw 'Task 3 requires pinned OpenCode CLI on PATH' }
if (-not (Get-Command python -ErrorAction SilentlyContinue)) { throw 'Task 3 requires Python on PATH' }

function Write-Utf8NoBom([string]$Path, [string]$Content) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force $parent | Out-Null }
    [System.IO.File]::WriteAllText($Path, $Content, [System.Text.UTF8Encoding]::new($false))
}

function Invoke-Git([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments) {
    & git @Arguments | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "git $($Arguments -join ' ') failed with exit code $LASTEXITCODE" }
}

function Assert-ExactKeys($Object, [string[]]$Expected, [string]$Name) {
    $actual = @($Object.PSObject.Properties.Name | Sort-Object)
    $want = @($Expected | Sort-Object)
    if (($actual -join '|') -ne ($want -join '|')) {
        throw "$Name keys invalid. actual=$($actual -join ',') expected=$($want -join ',')"
    }
}

$fixture = Join-Path $env:RUNNER_TEMP ("task162-hotfix-task3-plain-review-" + [guid]::NewGuid().ToString('N'))
$serverLog = Join-Path $env:RUNNER_TEMP ("task162-hotfix-task3-model-" + [guid]::NewGuid().ToString('N') + '.jsonl')
$transcript = Join-Path $env:RUNNER_TEMP ("task162-hotfix-task3-opencode-" + [guid]::NewGuid().ToString('N') + '.jsonl')
$serverProcess = $null

# Reserve a loopback port before the fixture baseline is committed so opencode.json
# itself is part of baseline state and cannot pollute the Runtime ChangeSet.
$listener = [System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback, 0)
$listener.Start()
$port = ([System.Net.IPEndPoint]$listener.LocalEndpoint).Port
$listener.Stop()

try {
    New-Item -ItemType Directory -Force $fixture | Out-Null
    Copy-Item (Join-Path $repoRoot '.code-harness') (Join-Path $fixture '.code-harness') -Recurse -Force

    Write-Utf8NoBom (Join-Path $fixture 'pom.xml') @'
<project>
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.acme</groupId>
  <artifactId>task162-hotfix-task3-fixture</artifactId>
  <version>1.0.0</version>
</project>
'@
    Write-Utf8NoBom (Join-Path $fixture 'src/main/resources/application.yml') @'
feature:
  task3: false
'@
    Write-Utf8NoBom (Join-Path $fixture '.code-harness/harness.yaml') @'
version: 2
project:
  type: maven
  root: .
  module: ""
review:
  baseRef: HEAD
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
        model = 'task3-local/task3'
        small_model = 'task3-local/task3'
        shell = 'pwsh'
        provider = @{
            'task3-local' = @{
                npm = '@ai-sdk/openai-compatible'
                name = 'Task3 Local Deterministic'
                options = @{
                    baseURL = "http://127.0.0.1:$port/v1"
                    apiKey = 'task3-local'
                }
                models = @{
                    task3 = @{
                        name = 'Task3 Deterministic'
                        limit = @{ context = 200000; output = 4096 }
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
description: Executes Codea Harness user intents through the active repository contracts
mode: primary
model: task3-local/task3
steps: 60
permission:
  read: allow
  edit: allow
  bash: allow
  webfetch: deny
  websearch: deny
---

You are the thin Agent Host adapter for Codea Harness acceptance testing.
For any user intent beginning with `harness`, first read `.code-harness/AGENTS.md`, then follow the active Harness Agent/Skill/Runtime contracts in this checkout.
Do not invent a parallel Harness protocol, do not bypass Controlled Runtime authority, and do not treat this host adapter as a source of Runtime request fields.
Complete the user's intent through the active contracts and only finish after the formal Harness artifact for that intent exists.
'@

    Push-Location $fixture
    try {
        Invoke-Git init
        Invoke-Git config user.email 'task162-task3@example.test'
        Invoke-Git config user.name 'Task 162 Task 3 E2E'
        Invoke-Git config core.autocrlf false
        Invoke-Git add .
        Invoke-Git commit -m 'baseline initialized Harness fixture'
        $baseHead = (git rev-parse HEAD).Trim()

        # This is the only review-relevant working-tree change seen by Canonical Snapshot.
        Write-Utf8NoBom (Join-Path $fixture 'src/main/resources/application.yml') @'
feature:
  task3: true
'@

        $reviewRelevant = @((git status --porcelain) | Where-Object { $_ -match 'src/main/resources/application.yml$' })
        if ($reviewRelevant.Count -ne 1) { throw "Task 3 fixture did not contain exactly one intended YML change: $($reviewRelevant -join '; ')" }

        $serverProcess = Start-Process -FilePath 'python' -ArgumentList @($modelServer, '--port', "$port", '--log', $serverLog) -PassThru -WindowStyle Hidden
        $healthy = $false
        for ($i = 0; $i -lt 40; $i++) {
            try {
                $health = Invoke-RestMethod -Uri "http://127.0.0.1:$port/health" -TimeoutSec 2
                if ($health.status -eq 'ok') { $healthy = $true; break }
            }
            catch { Start-Sleep -Milliseconds 250 }
        }
        if (-not $healthy) { throw 'Task 3 deterministic model server did not become healthy' }

        # The only review user message is deliberately the plain product intent.
        $plainUserIntent = 'harness review'
        $ErrorActionPreference = 'Continue'; $raw = (& opencode run --format json --auto --agent codea-harness-e2e --model task3-local/task3 $plainUserIntent 2>&1 | Out-String)
        $opencodeExit = $LASTEXITCODE; $ErrorActionPreference = 'Stop'
        Write-Utf8NoBom $transcript $raw
        if ($opencodeExit -ne 0) { throw "OpenCode plain harness review failed with exit ${opencodeExit}:`n$raw" }

        $runId = 'task3-plain-review'
        $runRoot = Join-Path $fixture ".code-harness\runs\$runId"
        $requestRoot = Join-Path $runRoot 'requests'
        $analysisRoot = Join-Path $runRoot 'analysis'
        $reportPath = Join-Path $runRoot 'review.md'

        $requiredArtifacts = @(
            (Join-Path $analysisRoot 'change-set.json'),
            (Join-Path $analysisRoot 'change-analysis.json'),
            (Join-Path $analysisRoot 'entrypoint-inventory.json'),
            (Join-Path $analysisRoot 'change-analysis.cert.json'),
            (Join-Path $analysisRoot 'review-options-origin.json'),
            (Join-Path $analysisRoot 'review-options.json'),
            (Join-Path $analysisRoot 'review-scope.json'),
            (Join-Path $analysisRoot 'review-units.json'),
            (Join-Path $analysisRoot 'rule-dispatch.json'),
            (Join-Path $analysisRoot 'certified-findings.json'),
            (Join-Path $analysisRoot 'certified-findings.cert.json'),
            $reportPath
        )
        foreach ($artifact in $requiredArtifacts) {
            if (!(Test-Path $artifact -PathType Leaf)) { throw "Task 3 real flow missing artifact: $artifact`nOpenCode transcript:`n$raw" }
        }

        $snapshotRequest = Get-Content -Raw (Join-Path $requestRoot 'change-set-request.json') | ConvertFrom-Json
        Assert-ExactKeys $snapshotRequest @('runId','baseRef','includeWorkingTree') 'Snapshot request'
        if ($snapshotRequest.PSObject.Properties.Name -contains 'requestedBaseRef') { throw 'Snapshot request illegally used Runtime provenance field requestedBaseRef' }
        if ([string]$snapshotRequest.baseRef -ne 'HEAD' -or -not [bool]$snapshotRequest.includeWorkingTree) { throw 'Snapshot request did not preserve effective review base/working-tree intent' }

        $proposal = Get-Content -Raw (Join-Path $requestRoot 'change-analysis-proposal.json') | ConvertFrom-Json
        if ($proposal.PSObject.Properties.Name -contains 'reviewScope' -or $proposal.PSObject.Properties.Name -contains 'changedFiles') {
            throw 'Semantic proposal attempted to carry Runtime-owned Git authority'
        }

        $certifyRequest = Get-Content -Raw (Join-Path $requestRoot 'analysis-certify-request.json') | ConvertFrom-Json
        Assert-ExactKeys $certifyRequest @('runId','snapshotPath','snapshotSha256','proposalPath','intent') 'Canonical certify request'
        Assert-ExactKeys $certifyRequest.intent @('mode') 'Canonical certify intent'
        if ([string]$certifyRequest.intent.mode -ne 'FULL') { throw "Canonical certify intent was not FULL: $($certifyRequest.intent.mode)" }
        if ($certifyRequest.PSObject.Properties.Name -contains 'draftPath' -or $certifyRequest.PSObject.Properties.Name -contains 'baseRef') { throw 'Canonical certify request fell back to legacy authority fields' }

        $snapshot = Get-Content -Raw (Join-Path $analysisRoot 'change-set.json') | ConvertFrom-Json
        if ([string]$certifyRequest.snapshotSha256 -ne [string]$snapshot.snapshotSha256) { throw 'Canonical certify request was not bound to Runtime Snapshot SHA' }
        $snapshotPaths = @($snapshot.files | ForEach-Object { [string]$_.path })
        if (($snapshotPaths -join '|') -ne 'src/main/resources/application.yml') { throw "Canonical Snapshot was polluted by E2E infrastructure: $($snapshotPaths -join ',')" }

        $inventory = Get-Content -Raw (Join-Path $analysisRoot 'entrypoint-inventory.json') | ConvertFrom-Json
        if ([string]$inventory.status -ne 'COMPLETE') { throw "Entrypoint inventory was not COMPLETE: $($inventory.status)" }

        $optionsRequest = Get-Content -Raw (Join-Path $requestRoot 'review-options-request.json') | ConvertFrom-Json
        Assert-ExactKeys $optionsRequest @('runId','changeAnalysisPath') 'Review Options request'
        if ($optionsRequest.PSObject.Properties.Name -contains 'baseRef') { throw 'Review Options request illegally injected baseRef' }
        $options = Get-Content -Raw (Join-Path $analysisRoot 'review-options.json') | ConvertFrom-Json
        if ([string]$options.decision -ne 'AUTO_FULL') { throw "Plain review did not resolve to deterministic AUTO_FULL: $($options.decision)" }

        $selectionRequest = Get-Content -Raw (Join-Path $requestRoot 'review-selection-request.json') | ConvertFrom-Json
        Assert-ExactKeys $selectionRequest @('runId','optionsHash','mode','selectionIds') 'Review selection request'
        if ([string]$selectionRequest.optionsHash -ne [string]$options.optionsHash -or [string]$selectionRequest.mode -ne 'FULL' -or @($selectionRequest.selectionIds).Count -ne 0) {
            throw 'AUTO_FULL selection was not bound to the current Runtime optionsHash'
        }

        $scope = Get-Content -Raw (Join-Path $analysisRoot 'review-scope.json') | ConvertFrom-Json
        if ([string]$scope.mode -ne 'FULL') { throw "Runtime scope was not FULL: $($scope.mode)" }
        $certifiedFindings = Get-Content -Raw (Join-Path $analysisRoot 'certified-findings.json') | ConvertFrom-Json
        if (@($certifiedFindings.findings).Count -ne 0) { throw 'Benign Task 3 fixture unexpectedly produced a Certified Finding' }

        $report = Get-Content -Raw $reportPath
        if ($report -notmatch '评审结果' -or $report -notmatch '通过') { throw "Final Runtime review.md was not a successful human report:`n$report" }

        $modelLogText = if (Test-Path $serverLog) { Get-Content -Raw $serverLog } else { '' }
        $combinedEvidence = $raw + "`n" + $modelLogText
        foreach ($requiredRead in @(
            '.code-harness/AGENTS.md',
            '.code-harness/agents/orchestrator.md',
            '.code-harness/agents/reviewer.md',
            '.code-harness/contracts/change-set-request.schema.json',
            '.code-harness/contracts/analysis-certify-request.schema.json',
            '.code-harness/contracts/review-options-request.schema.json'
        )) {
            if ($combinedEvidence -notmatch [regex]::Escape($requiredRead)) { throw "Agent Host did not prove active contract read: $requiredRead" }
        }
        foreach ($forbidden in @('REQUEST_CONTRACT_READ_FAILED','REQUEST_CONTRACT_INVALID','REQUEST_.*_SCHEMA_INVALID','REVIEW_OPTIONS_REQUEST_SCHEMA_INVALID','codea-harness-tools.exe','TASK3_E2E_ABORT')) {
            if ($combinedEvidence -match $forbidden) { throw "Task 3 detected forbidden contract-drift/legacy path: $forbidden" }
        }
        if ($combinedEvidence -notmatch 'harness review') { throw 'Model transcript did not contain the plain harness review user intent' }
        if ($combinedEvidence -notmatch 'TASK3_STAGE_21 PASS') { throw 'Agent Host did not complete the full real review chain' }

        Write-Output "TASK162_HOTFIX_TASK3_PLAIN_USER_INTENT PASS intent=$plainUserIntent"
        Write-Output 'TASK162_HOTFIX_TASK3_ACTIVE_CONTRACT_READ PASS'
        Write-Output 'TASK162_HOTFIX_TASK3_CANONICAL_SNAPSHOT_REQUEST PASS'
        Write-Output 'TASK162_HOTFIX_TASK3_CANONICAL_CERTIFY_REQUEST PASS'
        Write-Output 'TASK162_HOTFIX_TASK3_REVIEW_OPTIONS_NO_BASEREF PASS'
        Write-Output 'TASK162_HOTFIX_TASK3_AUTO_FULL_SCOPE PASS'
        Write-Output 'TASK162_HOTFIX_TASK3_RUNTIME_AUTHORITY_CHAIN PASS'
        Write-Output 'TASK162_HOTFIX_TASK3_FINAL_REVIEW_MD PASS'
        Write-Output 'TASK162_HOTFIX_TASK3_REAL_PLAIN_REVIEW_E2E PASS'
    }
    finally { Pop-Location }
}
finally {
    if ($null -ne $serverProcess -and -not $serverProcess.HasExited) { Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue }
    if (Test-Path $transcript) {
        Write-Output '--- Task 3 OpenCode transcript ---'
        Get-Content $transcript | Select-Object -Last 80
    }
    if (Test-Path $serverLog) {
        Write-Output '--- Task 3 model log tail ---'
        Get-Content $serverLog | Select-Object -Last 20
    }
    Remove-Item $fixture -Recurse -Force -ErrorAction SilentlyContinue
    
    
}
