$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$runtimeSource = Join-Path $repoRoot '.code-harness/bin/codea-dcep-tools.exe'
$astGrepSource = Join-Path $repoRoot '.code-harness/bin/ast-grep.exe'
$modelServer = Join-Path $PSScriptRoot 'task162-review-reliability-task1/mock_openai_server.py'
$artifactRoot = Join-Path $repoRoot '.task1-e2e-artifacts'

foreach ($required in @($runtimeSource, $astGrepSource, $modelServer)) {
    if (!(Test-Path $required -PathType Leaf)) { throw "Task 1 required file missing: $required" }
}
if (-not (Get-Command opencode -ErrorAction SilentlyContinue)) { throw 'Task 1 requires pinned OpenCode CLI on PATH' }
if (-not (Get-Command python -ErrorAction SilentlyContinue)) { throw 'Task 1 requires Python on PATH' }
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

function Assert-ExactKeys($Object, [string[]]$Expected, [string]$Name) {
    $actual = @($Object.PSObject.Properties.Name | Sort-Object)
    $want = @($Expected | Sort-Object)
    if (($actual -join '|') -ne ($want -join '|')) {
        throw "$Name keys invalid. actual=$($actual -join ',') expected=$($want -join ',')"
    }
}

function Invoke-ReviewScenario([ValidateSet('changed','zero')][string]$Scenario) {
    $runId = "task1-$Scenario-review"
    $fixture = Join-Path $env:RUNNER_TEMP ("task162-review-reliability-$Scenario-" + [guid]::NewGuid().ToString('N'))
    $serverLog = Join-Path $artifactRoot ("$Scenario-model.jsonl")
    $transcript = Join-Path $artifactRoot ("$Scenario-opencode.jsonl")
    $serverProcess = $null

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
  <artifactId>task162-review-reliability-fixture</artifactId>
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
            model = 'task1-local/task1'
            small_model = 'task1-local/task1'
            shell = 'pwsh'
            provider = @{
                'task1-local' = @{
                    npm = '@ai-sdk/openai-compatible'
                    name = 'Task1 Local Deterministic'
                    options = @{
                        baseURL = "http://127.0.0.1:$port/v1"
                        apiKey = 'task1-local'
                    }
                    models = @{
                        task1 = @{
                            name = 'Task1 Deterministic'
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
description: Executes Codea Harness intents through active repository contracts
mode: primary
model: task1-local/task1
steps: 80
permission:
  read: allow
  edit: allow
  bash: allow
  webfetch: deny
  websearch: deny
---

You are the thin Agent Host adapter for Codea Harness acceptance testing.
For every user intent beginning with `harness`, read the active Harness contracts in this checkout and obey their Runtime request schemas and authority boundaries.
Do not invent a parallel protocol. Do not bypass Controlled Runtime authority. Complete the user's intent only after the formal Harness artifact exists.
'@

        Push-Location $fixture
        try {
            Invoke-Git init
            Invoke-Git config user.email 'task162-task1@example.test'
            Invoke-Git config user.name 'Task 162 Task 1 E2E'
            Invoke-Git config core.autocrlf false
            Invoke-Git add .
            Invoke-Git commit -m 'baseline review reliability fixture'

            if ($Scenario -eq 'changed') {
                Write-Utf8NoBom (Join-Path $fixture 'src/main/resources/application.yml') @'
feature:
  reliability: true
'@
                $reviewRelevant = @((git status --porcelain) | Where-Object { $_ -match 'src/main/resources/application.yml$' })
                if ($reviewRelevant.Count -ne 1) { throw "Changed scenario did not contain exactly one intended YML change: $($reviewRelevant -join '; ')" }
            }
            else {
                $reviewRelevant = @((git status --porcelain) | Where-Object { $_ -match 'src/(main|test)/' })
                if ($reviewRelevant.Count -ne 0) { throw "Zero scenario unexpectedly contains review-relevant change: $($reviewRelevant -join '; ')" }
            }

            $serverProcess = Start-Process -FilePath 'python' -ArgumentList @($modelServer, '--port', "$port", '--log', $serverLog, '--scenario', $Scenario) -PassThru -WindowStyle Hidden
            $healthy = $false
            for ($i = 0; $i -lt 40; $i++) {
                try {
                    $health = Invoke-RestMethod -Uri "http://127.0.0.1:$port/health" -TimeoutSec 2
                    if ($health.status -eq 'ok') { $healthy = $true; break }
                }
                catch { Start-Sleep -Milliseconds 250 }
            }
            if (-not $healthy) { throw "Task 1 $Scenario model server did not become healthy" }

            $plainUserIntent = 'harness review'
            $ErrorActionPreference = 'Continue'
            $raw = (& opencode run --format json --auto --agent codea-harness-e2e --model task1-local/task1 $plainUserIntent 2>&1 | Out-String)
            $opencodeExit = $LASTEXITCODE
            $ErrorActionPreference = 'Stop'
            Write-Utf8NoBom $transcript $raw
            if ($opencodeExit -ne 0) { throw "OpenCode $Scenario harness review failed with exit ${opencodeExit}:`n$raw" }

            $runRoot = Join-Path $fixture ".code-harness/runs/$runId"
            $requestRoot = Join-Path $runRoot 'requests'
            $analysisRoot = Join-Path $runRoot 'analysis'
            $reportPath = Join-Path $runRoot 'review.md'
            $requiredArtifacts = @(
                (Join-Path $analysisRoot 'change-set.json'),
                (Join-Path $analysisRoot 'change-analysis.json'),
                (Join-Path $analysisRoot 'entrypoint-inventory.json'),
                (Join-Path $analysisRoot 'change-analysis.cert.json'),
                (Join-Path $analysisRoot 'review-options.json'),
                (Join-Path $analysisRoot 'review-scope.json'),
                (Join-Path $analysisRoot 'review-units.json'),
                (Join-Path $analysisRoot 'rule-dispatch.json'),
                (Join-Path $analysisRoot 'certified-findings.json'),
                (Join-Path $analysisRoot 'certified-findings.cert.json'),
                $reportPath
            )
            foreach ($artifact in $requiredArtifacts) {
                if (!(Test-Path $artifact -PathType Leaf)) { throw "Task 1 $Scenario flow missing artifact: $artifact`nOpenCode transcript:`n$raw" }
            }

            $snapshot = Get-Content -Raw (Join-Path $analysisRoot 'change-set.json') | ConvertFrom-Json
            $snapshotPaths = @($snapshot.files | ForEach-Object { [string]$_.path })
            if ($Scenario -eq 'changed') {
                if (($snapshotPaths -join '|') -ne 'src/main/resources/application.yml') { throw "Changed Snapshot mismatch: $($snapshotPaths -join ',')" }
            }
            else {
                if ($snapshotPaths.Count -ne 0) { throw "Zero Snapshot contains changes: $($snapshotPaths -join ',')" }
            }

            $findingRequest = Get-Content -Raw (Join-Path $requestRoot 'finding-certify-request.json') | ConvertFrom-Json
            Assert-ExactKeys $findingRequest @('runId','proposalsPath') "Task 1 $Scenario Finding Certify request"
            if ([string]$findingRequest.runId -ne $runId -or [string]$findingRequest.proposalsPath -ne ".code-harness/runs/$runId/requests/finding-proposals.json") {
                throw "Task 1 $Scenario Finding Certify request is not canonical"
            }

            $proposals = Get-Content -Raw (Join-Path $requestRoot 'finding-proposals.json') | ConvertFrom-Json
            if (@($proposals).Count -ne 0) { throw "Task 1 $Scenario fixture unexpectedly produced Finding proposals" }
            $certified = Get-Content -Raw (Join-Path $analysisRoot 'certified-findings.json') | ConvertFrom-Json
            if (@($certified.findings).Count -ne 0) { throw "Task 1 $Scenario fixture unexpectedly produced Certified Findings" }

            if ($Scenario -eq 'zero') {
                $units = Get-Content -Raw (Join-Path $analysisRoot 'review-units.json') | ConvertFrom-Json
                $dispatch = Get-Content -Raw (Join-Path $analysisRoot 'rule-dispatch.json') | ConvertFrom-Json
                if (@($units.units).Count -ne 0) { throw 'Zero Change must produce an empty ReviewUnit manifest' }
                if (@($dispatch.dispatches).Count -ne 0) { throw 'Zero Change must produce an empty Rule Dispatch manifest' }
            }

            if (Test-Path (Join-Path $requestRoot 'report-review.json')) { throw "Task 1 $Scenario report transport was not deleted after successful Runtime consumption" }
            $report = Get-Content -Raw $reportPath
            if ($report -notmatch '评审结果' -or $report -notmatch '通过') { throw "Task 1 $Scenario final review.md is not PASSED:`n$report" }
            if ($Scenario -eq 'zero') {
                if ($report -notmatch '无变更文件') { throw "Zero Change review.md did not explain the empty ChangeSet:`n$report" }
                if ($report -notmatch '问题数量[^\r\n]*0') { throw "Zero Change review.md did not show zero findings:`n$report" }
            }

            $modelLogText = if (Test-Path $serverLog) { Get-Content -Raw $serverLog } else { '' }
            $combinedEvidence = $raw + "`n" + $modelLogText
            foreach ($requiredCommand in @(
                'codea-dcep-tools.exe review options --input',
                'codea-dcep-tools.exe review select --input',
                'codea-dcep-tools.exe review units --run-id',
                'codea-dcep-tools.exe review dispatch --run-id',
                'codea-dcep-tools.exe review certify-findings --input',
                'codea-dcep-tools.exe report review --input'
            )) {
                if ($combinedEvidence -notmatch [regex]::Escape($requiredCommand)) { throw "Task 1 $Scenario Agent Host did not discover active Runtime command: $requiredCommand" }
            }
            foreach ($requiredRead in @(
                '.code-harness/AGENTS.md',
                '.code-harness/tools/README.md',
                '.code-harness/agents/orchestrator.md',
                '.code-harness/agents/reviewer.md',
                '.code-harness/contracts/finding-certify-request.schema.json',
                '.code-harness/contracts/report-review-request.schema.json'
            )) {
                if ($combinedEvidence -notmatch [regex]::Escape($requiredRead)) { throw "Task 1 $Scenario did not prove active contract/schema read: $requiredRead" }
            }
            $findingSchemaIndex = $combinedEvidence.IndexOf('TASK1_FINDING_SCHEMA_READ')
            $findingWriteIndex = $combinedEvidence.IndexOf('TASK1_FINDING_REQUEST_WRITTEN')
            if ($findingSchemaIndex -lt 0 -or $findingWriteIndex -lt 0 -or $findingSchemaIndex -ge $findingWriteIndex) {
                throw "Task 1 $Scenario did not prove Finding Certify schema-read-before-request"
            }
            $reportSchemaIndex = $combinedEvidence.IndexOf('TASK1_REPORT_SCHEMA_READ')
            $reportWriteIndex = $combinedEvidence.IndexOf('TASK1_REPORT_REQUEST_WRITTEN')
            if ($reportSchemaIndex -lt 0 -or $reportWriteIndex -lt 0 -or $reportSchemaIndex -ge $reportWriteIndex) {
                throw "Task 1 $Scenario did not prove Report Review schema-read-before-request"
            }
            foreach ($forbidden in @(
                'unknown field',
                'cannot unmarshal array',
                'FINDING_CERTIFY_REQUEST_SCHEMA_INVALID',
                'REPORT_REVIEW_REQUEST_SCHEMA_INVALID',
                'Runtime 不支持 certify-findings',
                '缺少 certified-findings',
                'TASK1_E2E_ABORT'
            )) {
                if ($combinedEvidence -match [regex]::Escape($forbidden)) { throw "Task 1 $Scenario detected forbidden failure text: $forbidden" }
            }
            if ($combinedEvidence -notmatch 'TASK1_STAGE_23 PASS') { throw "Task 1 $Scenario Agent Host did not complete the full authority chain" }

            Write-Output "TASK162_REVIEW_RELIABILITY_TASK1_${($Scenario.ToUpper())}_ACTIVE_CONTRACT_DISCOVERY PASS"
            Write-Output "TASK162_REVIEW_RELIABILITY_TASK1_${($Scenario.ToUpper())}_SCHEMA_READ_BEFORE_REQUEST PASS"
            Write-Output "TASK162_REVIEW_RELIABILITY_TASK1_${($Scenario.ToUpper())}_CERTIFIED_FINDINGS PASS"
            Write-Output "TASK162_REVIEW_RELIABILITY_TASK1_${($Scenario.ToUpper())}_FINAL_REVIEW PASS"
        }
        finally { Pop-Location }
    }
    finally {
        if ($null -ne $serverProcess -and -not $serverProcess.HasExited) { Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue }
        Remove-Item $fixture -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Invoke-ReviewScenario -Scenario changed
Invoke-ReviewScenario -Scenario zero
Write-Output 'TASK162_REVIEW_RELIABILITY_TASK1_REAL_AGENT_E2E PASS'
