$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$runtimeSource = Join-Path $repoRoot '.code-harness\bin\codea-dcep-tools.exe'
$astGrepSource = Join-Path $repoRoot '.code-harness\bin\ast-grep.exe'
$modelServer = Join-Path $PSScriptRoot 'task163-task3\mock_openai_server.py'
foreach ($required in @($runtimeSource,$astGrepSource,$modelServer)) {
    if (!(Test-Path $required -PathType Leaf)) { throw "Task 3 required file missing: $required" }
}
if (-not (Get-Command opencode -ErrorAction SilentlyContinue)) { throw 'Task 3 requires pinned OpenCode CLI on PATH' }
if (-not (Get-Command python -ErrorAction SilentlyContinue)) { throw 'Task 3 requires Python on PATH' }

function Write-Utf8NoBom([string]$Path,[string]$Content) {
    $parent=Split-Path -Parent $Path
    if($parent){New-Item -ItemType Directory -Force $parent|Out-Null}
    [System.IO.File]::WriteAllText($Path,$Content,[System.Text.UTF8Encoding]::new($false))
}
function Invoke-Git([Parameter(ValueFromRemainingArguments=$true)][string[]]$Arguments) {
    & git @Arguments | Out-Null
    if($LASTEXITCODE-ne 0){throw "git $($Arguments -join ' ') failed"}
}
function Assert-Absent([string]$Path,[string]$Label) {
    if(Test-Path $Path){throw "Turn 1 illegally produced $Label at $Path"}
}

$fixture=Join-Path $env:RUNNER_TEMP ("task163-task3-multichain-"+[guid]::NewGuid().ToString('N'))
$serverLog=Join-Path $env:RUNNER_TEMP ("task163-task3-model-"+[guid]::NewGuid().ToString('N')+'.jsonl')
$turn1Transcript=Join-Path $env:RUNNER_TEMP ("task163-task3-turn1-"+[guid]::NewGuid().ToString('N')+'.jsonl')
$turn2Transcript=Join-Path $env:RUNNER_TEMP ("task163-task3-turn2-"+[guid]::NewGuid().ToString('N')+'.jsonl')
$serverProcess=$null
$listener=[System.Net.Sockets.TcpListener]::new([System.Net.IPAddress]::Loopback,0)
$listener.Start();$port=([System.Net.IPEndPoint]$listener.LocalEndpoint).Port;$listener.Stop()

try {
    New-Item -ItemType Directory -Force $fixture|Out-Null
    Copy-Item (Join-Path $repoRoot '.code-harness') (Join-Path $fixture '.code-harness') -Recurse -Force

    Write-Utf8NoBom (Join-Path $fixture 'pom.xml') @'
<project><modelVersion>4.0.0</modelVersion><groupId>com.acme</groupId><artifactId>task163-task3</artifactId><version>1.0.0</version></project>
'@
    Write-Utf8NoBom (Join-Path $fixture 'src/main/java/com/acme/order/OrderController.java') @'
package com.acme.order;
@RestController
public class OrderController {
    private final OrderService service = new OrderService();
    @PostMapping("/approve")
    public int approve() { return service.approve(); }
}
'@
    Write-Utf8NoBom (Join-Path $fixture 'src/main/java/com/acme/order/OrderService.java') @'
package com.acme.order;
@Service
public class OrderService { public int approve() { return 1; } }
'@
    Write-Utf8NoBom (Join-Path $fixture 'src/main/java/com/acme/payment/PaymentController.java') @'
package com.acme.payment;
@RestController
public class PaymentController {
    private final PaymentService service = new PaymentService();
    @PostMapping("/pay")
    public int pay() { return service.pay(); }
}
'@
    Write-Utf8NoBom (Join-Path $fixture 'src/main/java/com/acme/payment/PaymentService.java') @'
package com.acme.payment;
@Service
public class PaymentService { public int pay() { return 1; } }
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

    $config=@{
        '$schema'='https://opencode.ai/config.json';model='task3-local/task3';small_model='task3-local/task3';shell='pwsh'
        provider=@{'task3-local'=@{npm='@ai-sdk/openai-compatible';name='Task3 Local Deterministic';options=@{baseURL="http://127.0.0.1:$port/v1";apiKey='task3-local'};models=@{task3=@{name='Task3 Deterministic';limit=@{context=200000;output=4096}}}}}
        permission=@{read='allow';edit='allow';bash='allow';webfetch='deny';websearch='deny'}
    }
    Write-Utf8NoBom (Join-Path $fixture 'opencode.json') ($config|ConvertTo-Json -Depth 20)
    Write-Utf8NoBom (Join-Path $fixture '.opencode/agents/codea-harness-e2e.md') @'
---
description: Executes Codea Harness intents only through active repository contracts
mode: primary
model: task3-local/task3
steps: 80
permission:
  read: allow
  edit: allow
  bash: allow
  webfetch: deny
  websearch: deny
---
You are a thin Agent Host adapter for Codea Harness acceptance testing.
For every `harness` intent, read `.code-harness/AGENTS.md` and follow the active Orchestrator/Reviewer/Skill/Runtime contracts. Do not invent a parallel protocol or bypass Controlled Runtime authority.
'@

    Push-Location $fixture
    try {
        Invoke-Git init
        Invoke-Git config user.email 'task163-task3@example.test'
        Invoke-Git config user.name 'Task 163 Task 3 E2E'
        Invoke-Git config core.autocrlf false
        Invoke-Git add .
        Invoke-Git commit -m 'baseline multi-chain fixture'

        # Two actual changed Controller methods create two independent confirmed business chains.
        (Get-Content -Raw 'src/main/java/com/acme/order/OrderController.java').Replace('return service.approve();','return service.approve() + 0;') | Set-Content -NoNewline -Encoding utf8 'src/main/java/com/acme/order/OrderController.java'
        (Get-Content -Raw 'src/main/java/com/acme/payment/PaymentController.java').Replace('return service.pay();','return service.pay() + 0;') | Set-Content -NoNewline -Encoding utf8 'src/main/java/com/acme/payment/PaymentController.java'

        $serverProcess=Start-Process -FilePath 'python' -ArgumentList @($modelServer,'--port',"$port",'--log',$serverLog) -PassThru -WindowStyle Hidden
        $healthy=$false
        for($i=0;$i-lt 40;$i++){
            try{$h=Invoke-RestMethod -Uri "http://127.0.0.1:$port/health" -TimeoutSec 2;if($h.status-eq'ok'){$healthy=$true;break}}catch{Start-Sleep -Milliseconds 250}
        }
        if(-not $healthy){throw 'Task 3 model server did not become healthy'}

        # Turn 1: exact product intent. It MUST stop after USER_SELECTION.
        $ErrorActionPreference='Continue'
        $turn1=(& opencode run --format json --auto --agent codea-harness-e2e --model task3-local/task3 'harness review' 2>&1 | Out-String)
        $turn1Exit=$LASTEXITCODE
        $ErrorActionPreference='Stop'
        Write-Utf8NoBom $turn1Transcript $turn1
        if($turn1Exit-ne 0){throw "Turn 1 OpenCode failed ${turn1Exit}:`n$turn1"}

        $run='.code-harness/runs/task3-multi-chain-review'
        $optionsPath=Join-Path $run 'analysis/review-options.json'
        if(!(Test-Path $optionsPath)){throw 'Turn 1 did not produce review-options.json'}
        $options=Get-Content -Raw $optionsPath|ConvertFrom-Json
        if([string]$options.decision-ne'USER_SELECTION'){throw "Turn 1 decision=$($options.decision), expected USER_SELECTION"}
        if(@($options.chains).Count-lt 2){throw 'Turn 1 must expose 2+ Runtime chain options'}
        if($turn1-notmatch 'TURN1_SELECTION_REQUIRED'){throw "Turn 1 did not ask the user:`n$turn1"}

        Assert-Absent (Join-Path $run 'requests/review-selection-request.json') 'review select request'
        Assert-Absent (Join-Path $run 'analysis/review-scope.json') 'review scope'
        Assert-Absent (Join-Path $run 'analysis/review-units.json') 'review units'
        Assert-Absent (Join-Path $run 'analysis/rule-dispatch.json') 'review dispatch'
        Assert-Absent (Join-Path $run 'requests/finding-proposals.json') 'finding proposals'
        Assert-Absent (Join-Path $run 'analysis/certified-findings.json') 'certified findings'
        Assert-Absent (Join-Path $run 'review.md') 'review report'
        Write-Output 'MULTI_CHAIN_REVIEW_REQUIRES_USER_SELECTION PASS'
        Write-Output 'MULTI_CHAIN_NO_SELECTION_NO_REVIEW_UNITS PASS'
        Write-Output 'TASK163_TASK3_TURN1_HARD_STOP PASS'

        # Turn 2: continue the same OpenCode Session, now with explicit user selection.
        $ErrorActionPreference='Continue'
        $turn2=(& opencode run --continue --format json --auto --agent codea-harness-e2e --model task3-local/task3 '全部' 2>&1 | Out-String)
        $turn2Exit=$LASTEXITCODE
        $ErrorActionPreference='Stop'
        Write-Utf8NoBom $turn2Transcript $turn2
        if($turn2Exit-ne 0){throw "Turn 2 OpenCode failed ${turn2Exit}:`n$turn2"}

        foreach($artifact in @(
            (Join-Path $run 'requests/review-selection-request.json'),
            (Join-Path $run 'analysis/review-scope.json'),
            (Join-Path $run 'analysis/review-units.json'),
            (Join-Path $run 'analysis/rule-dispatch.json'),
            (Join-Path $run 'analysis/certified-findings.json'),
            (Join-Path $run 'analysis/certified-findings.cert.json'),
            (Join-Path $run 'review.md')
        )){if(!(Test-Path $artifact)){throw "Turn 2 missing artifact: $artifact"}}

        $selection=Get-Content -Raw (Join-Path $run 'requests/review-selection-request.json')|ConvertFrom-Json
        if([string]$selection.mode-ne'FULL' -or [string]$selection.optionsHash-ne[string]$options.optionsHash -or @($selection.selectionIds).Count-ne 0){throw 'Turn 2 FULL selection not bound to Turn 1 optionsHash'}
        $scope=Get-Content -Raw (Join-Path $run 'analysis/review-scope.json')|ConvertFrom-Json
        if([string]$scope.mode-ne'FULL'){throw 'Turn 2 Runtime scope was not FULL'}
        $report=Get-Content -Raw (Join-Path $run 'review.md')
        if($report-notmatch '评审结果' -or $report-notmatch '通过'){throw "Turn 2 final review report invalid:`n$report"}

        $modelLog=Get-Content -Raw $serverLog
        if($modelLog-notmatch '"event": "turn2_resume"' -or $modelLog-notmatch '"sameSession": true'){throw 'Model did not prove same-session Turn 2 context'}
        if(($turn1+"`n"+$turn2+"`n"+$modelLog)-notmatch 'TASK163_STAGE_21 PASS'){throw 'Turn 2 did not complete full Runtime authority chain'}
        Write-Output 'TASK163_TASK3_SAME_SESSION_CONTEXT PASS'
        Write-Output 'TASK163_TASK3_TURN2_USER_FULL_SELECTION PASS'
        Write-Output 'TASK163_TASK3_RUNTIME_AUTHORITY_CHAIN PASS'
        Write-Output 'TASK163_TASK3_REAL_OPENCODE_SAME_SESSION_E2E PASS'
    }
    finally{Pop-Location}
}
finally{
    if($null-ne$serverProcess -and -not $serverProcess.HasExited){Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue}
    foreach($path in @($serverLog,$turn1Transcript,$turn2Transcript)){
        if(Test-Path $path){Write-Host "TASK163_EVIDENCE_FILE $path"}
    }
}
