$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$installZip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-install.zip'
$modelServer = Join-Path $PSScriptRoot 'task164-task4-plain-review-server.py'
foreach ($required in @($installZip,$modelServer)) {
    if (-not (Test-Path $required -PathType Leaf)) { throw "Task 4 interruption E2E missing required file: $required" }
}
foreach ($command in @('opencode','python','git')) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) { throw "Task 4 interruption E2E requires $command" }
}

$utf8 = [Text.UTF8Encoding]::new($false)
function Write-Utf8NoBom([string]$Path, [string]$Content) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force $parent | Out-Null }
    [IO.File]::WriteAllText($Path, $Content, $utf8)
}
function Invoke-Git([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments) {
    & git @Arguments | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "git $($Arguments -join ' ') failed with exit code $LASTEXITCODE" }
}

$fixture = Join-Path $env:RUNNER_TEMP ('task164-task4-interruption-' + [guid]::NewGuid().ToString('N'))
$evidenceRoot = if (-not [string]::IsNullOrWhiteSpace($env:TASK164_FINAL_EVIDENCE_DIR)) {
    Join-Path $env:TASK164_FINAL_EVIDENCE_DIR 'task4-progress-interruption'
} else {
    Join-Path $env:RUNNER_TEMP ('task164-task4-interruption-evidence-' + [guid]::NewGuid().ToString('N'))
}
New-Item -ItemType Directory -Force $fixture,$evidenceRoot | Out-Null
Expand-Archive -Path $installZip -DestinationPath $fixture -Force
foreach ($required in @(
    '.code-harness/bin/codea-dcep-tools.exe',
    '.code-harness/bootstrap.md',
    '.code-harness/AGENTS.md',
    '.code-harness/agents/orchestrator.md',
    '.opencode/agents/reviewer.md',
    '.opencode/commands/harness-review-reviewer.md',
    '.opencode/tools/codea-reviewer-submit.ts'
)) {
    if (-not (Test-Path (Join-Path $fixture $required) -PathType Leaf)) { throw "packaged interruption resource missing: $required" }
}

Write-Utf8NoBom (Join-Path $fixture '.gitignore') ".code-harness/`n.opencode/`nopencode.json`n"
$sourcePath = Join-Path $fixture 'src/main/resources/application.yml'
New-Item -ItemType Directory -Force (Split-Path -Parent $sourcePath) | Out-Null
Write-Utf8NoBom $sourcePath "interruption: false`n"
Push-Location $fixture
try {
    Invoke-Git init -b develop
    Invoke-Git config user.email 'task164-interruption@example.test'
    Invoke-Git config user.name 'Task164 Task4 Interruption E2E'
    Invoke-Git config core.autocrlf false
    Invoke-Git add .gitignore src
    Invoke-Git commit -m 'Task 4 interruption baseline'
}
finally { Pop-Location }
Write-Utf8NoBom $sourcePath "interruption: true`n"

$listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
$listener.Start()
$port = ([Net.IPEndPoint]$listener.LocalEndpoint).Port
$listener.Stop()
$modelLog = Join-Path $evidenceRoot 'model.jsonl'
$transcript = Join-Path $evidenceRoot 'opencode-run.jsonl'
$config = @{
    '$schema' = 'https://opencode.ai/config.json'
    model = 'task4-local/task4'
    small_model = 'task4-local/task4'
    instructions = @('.code-harness/bootstrap.md','.code-harness/AGENTS.md','.code-harness/agents/orchestrator.md')
    provider = @{
        'task4-local' = @{
            npm = '@ai-sdk/openai-compatible'
            name = 'Task 4 Interruption Deterministic'
            options = @{ baseURL = "http://127.0.0.1:$port/v1"; apiKey = 'task164-local' }
            models = @{ 'task4' = @{ name = 'Task 4 Deterministic'; tool_call = $true; limit = @{ context = 200000; output = 4096 } } }
        }
    }
    permission = @{ '*'='deny'; read='allow'; edit='allow'; bash='allow'; task='allow' }
} | ConvertTo-Json -Depth 20
Write-Utf8NoBom (Join-Path $fixture 'opencode.json') $config

$serverProcess = Start-Process -FilePath python -ArgumentList @($modelServer,'--port',"$port",'--log',$modelLog,'--scenario','reviewer-unavailable') -PassThru -WindowStyle Hidden
$passed = $false
try {
    $healthy = $false
    for ($i=0; $i -lt 60; $i++) {
        try {
            $health = Invoke-RestMethod -Uri "http://127.0.0.1:$port/health" -TimeoutSec 1
            if ($health.status -eq 'ok' -and $health.scenario -eq 'reviewer-unavailable') { $healthy=$true; break }
        } catch { Start-Sleep -Milliseconds 100 }
    }
    if (-not $healthy) { throw 'Task 4 interruption deterministic provider did not start' }

    Push-Location $fixture
    try {
        $ErrorActionPreference='Continue'
        $raw = (& opencode run --format json --auto --model task4-local/task4 harness review 2>&1 | Out-String)
        $exit = $LASTEXITCODE
        $ErrorActionPreference='Stop'
    }
    finally { Pop-Location }
    Write-Utf8NoBom $transcript $raw
    # A controlled hard stop is a product outcome, not a requirement that the
    # OpenCode process itself return a particular transport exit code.
    foreach ($marker in @('REVIEWER_UNAVAILABLE','MANUAL_ACTION_REQUIRED','HARD STOP')) {
        if (-not $raw.Contains($marker)) { throw "OpenCode interruption transcript missing $marker; exit=$exit`n$raw" }
    }
    if ($raw.Contains('TASK4_FULL_REVIEW_COMPLETE')) { throw 'interruption path fell through to semantic success' }
    if ($raw.Contains(('TASK4' + '_STAGE_'))) { throw 'prompt-only stage markers leaked into interruption transcript' }

    $runDirs = @(Get-ChildItem (Join-Path $fixture '.code-harness/runs') -Directory)
    if ($runDirs.Count -ne 1) {
        $runNames = @($runDirs | ForEach-Object { [string]$_.Name })
        $modelText = ''
        if (Test-Path $modelLog -PathType Leaf) { $modelText = Get-Content -Raw $modelLog }
        throw "interruption must create exactly one Runtime run; count=$($runDirs.Count); found=$($runNames -join ',')`nTRANSCRIPT:`n$raw`nMODEL_LOG:`n$modelText"
    }
    $runId = $runDirs[0].Name
    $runRoot = $runDirs[0].FullName
    $progressPath = Join-Path $runRoot 'runtime/review-progress.json'
    if (-not (Test-Path $progressPath -PathType Leaf)) { throw 'interruption missing Runtime progress artifact' }
    $progressRaw = Get-Content -Raw $progressPath
    $after = $progressRaw | ConvertFrom-Json
    if ($after.status -ne 'FAILED' -or $after.currentStage -ne 'CHANGE_ANALYSIS' -or $after.failureStage -ne 'CHANGE_ANALYSIS' -or $after.failureCode -ne 'REVIEWER_UNAVAILABLE') {
        throw "Runtime did not attribute product failure to CHANGE_ANALYSIS`n$progressRaw"
    }
    $stages = @($after.stages)
    $expected = @(
        @('REVIEW_BEGIN','SUCCEEDED'),
        @('SNAPSHOT','SUCCEEDED'),
        @('CHANGE_ANALYSIS','FAILED'),
        @('CERTIFICATION','BLOCKED'),
        @('REVIEW_PLANNING','BLOCKED'),
        @('REVIEW_EXECUTION','BLOCKED'),
        @('FINDING_CERTIFICATION','BLOCKED'),
        @('REPORT','BLOCKED')
    )
    if ($stages.Count -ne $expected.Count) { throw "expected 8 Runtime stages, got $($stages.Count)" }
    for ($i=0; $i -lt $expected.Count; $i++) {
        if ([string]$stages[$i].name -ne $expected[$i][0] -or [string]$stages[$i].status -ne $expected[$i][1]) {
            throw "unexpected Runtime stage[$i]: $($stages[$i].name)/$($stages[$i].status)"
        }
    }
    $failureEvents = @($after.events | Where-Object { $_.stage -eq 'CHANGE_ANALYSIS' -and $_.status -eq 'FAILED' -and $_.failureCode -eq 'REVIEWER_UNAVAILABLE' })
    if ($failureEvents.Count -ne 1) { throw 'missing unique CHANGE_ANALYSIS FAILED Runtime event' }
    $visibleEvents = @($after.events | Where-Object { $_.status -in @('FAILED','BLOCKED') })
    foreach ($event in $visibleEvents) {
        $display = [string]$event.display
        if ([string]::IsNullOrWhiteSpace($display)) { throw "Runtime interruption event missing display for $($event.stage)" }
        if (-not $raw.Contains($display)) { throw "OpenCode transcript did not render Runtime interruption event: $display" }
    }
    Write-Output "TASK164_TASK4_INTERRUPTION_ARMED PASS runId=$runId currentStage=CHANGE_ANALYSIS"
    Write-Output 'OPENCODE_INTERRUPTION_STAGE_VISIBLE PASS'
    Write-Output 'OPENCODE_INTERRUPTION_LATER_STAGES_BLOCKED PASS'

    foreach ($forbidden in @(
        'requests/change-analysis-proposal.json',
        'requests/change-analysis-reviewer-authority.json',
        'analysis/change-analysis.json',
        'analysis/change-analysis.cert.json',
        'analysis/review-options.json',
        'analysis/review-scope.json',
        'analysis/review-units.json',
        'analysis/rule-dispatch.json',
        'requests/finding-proposals.json',
        'requests/finding-reviewer-authority.json',
        'analysis/certified-findings.json',
        'analysis/certified-findings.cert.json',
        'review.md'
    )) {
        if (Test-Path (Join-Path $runRoot $forbidden)) { throw "downstream artifact exists after product interruption: $forbidden" }
    }

    $modelEntries = @(
        foreach ($line in @(Get-Content $modelLog)) {
            if (-not [string]::IsNullOrWhiteSpace($line)) { $line | ConvertFrom-Json }
        }
    )
    $toolResponses = @($modelEntries | Where-Object { $_.PSObject.Properties.Name -contains 'responseType' -and [string]$_.responseType -eq 'tool' })
    $taskCalls = @($toolResponses | Where-Object { $_.tool -eq 'task' })
    if ($taskCalls.Count -ne 1 -or [string]$taskCalls[0].arguments.subagent_type -ne 'reviewer') {
        throw 'interruption must attempt exactly one independent Reviewer delegation and no fallback task'
    }
    $bashCalls = @($toolResponses | Where-Object { $_.tool -eq 'bash' })
    $unavailable = ".code-harness/bin/codea-dcep-tools.exe review reviewer-unavailable --run-id $runId"
    if (@($bashCalls | Where-Object { [string]$_.arguments.command -eq $unavailable }).Count -ne 1) {
        throw 'Main Agent did not report Reviewer failure through the official Runtime command exactly once'
    }
    foreach ($entry in $bashCalls) {
        $command = [string]$entry.arguments.command
        if ($command -match '(?i)(powershell|pwsh|new-item|writealltext|convertfrom-json|out-string|\||>|<|;|&&|\$\(|`)') {
            throw "interruption E2E used forbidden shell orchestration: $command"
        }
    }
    if (@($toolResponses | Where-Object { $_.tool -like '*reviewer*submit*' }).Count -ne 0) {
        throw 'controlled unavailable Reviewer unexpectedly submitted semantic output'
    }

    Write-Utf8NoBom (Join-Path $evidenceRoot 'review-progress.json') $progressRaw
    Write-Output "TASK164_TASK4_INTERRUPTION_CHANGE_ANALYSIS PASS runId=$runId failureCode=$($after.failureCode)"
    Write-Output 'TASK164_TASK4_DOWNSTREAM_BLOCKED PASS certification=BLOCKED planning=BLOCKED execution=BLOCKED findingCertification=BLOCKED report=BLOCKED'
    Write-Output 'TASK164_TASK4_GATE_D PASS'
    $passed=$true
}
finally {
    if ($serverProcess -and -not $serverProcess.HasExited) { Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue }
    if ($passed) { Remove-Item $fixture -Recurse -Force -ErrorAction SilentlyContinue }
    else { Write-Warning "Task 4 interruption failure fixture retained at $fixture; evidence=$evidenceRoot" }
}
