$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$installZip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-install.zip'
$modelServer = Join-Path $PSScriptRoot 'task164-task4-plain-review-server.py'
foreach ($required in @($installZip, $modelServer)) {
    if (-not (Test-Path $required -PathType Leaf)) { throw "Task 4 packaged review E2E missing required file: $required" }
}
foreach ($command in @('opencode','python','git')) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) { throw "Task 4 packaged review E2E requires $command on PATH" }
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

function Assert-InOrder([string]$Text, [string[]]$Markers, [string]$Name) {
    $previous = -1
    foreach ($marker in $Markers) {
        $index = $Text.IndexOf($marker, [StringComparison]::Ordinal)
        if ($index -lt 0) { throw "$Name missing marker: $marker" }
        if ($index -le $previous) { throw "$Name marker out of order: $marker" }
        $previous = $index
    }
}

function Invoke-OpenCodeExport([string]$SessionId) {
    $stderrPath = Join-Path $env:RUNNER_TEMP ("task164-task4-export-$SessionId-" + [guid]::NewGuid().ToString('N') + '.stderr.log')
    try {
        $stdout = (& opencode export $SessionId 2> $stderrPath | Out-String)
        $exit = $LASTEXITCODE
        if ($exit -ne 0) {
            $stderr = ''
            if (Test-Path $stderrPath -PathType Leaf) { $stderr = Get-Content -Raw $stderrPath }
            throw "opencode export $SessionId failed exit=$exit`n$stderr"
        }
        return $stdout
    }
    finally { Remove-Item $stderrPath -Force -ErrorAction SilentlyContinue }
}

$fixture = Join-Path $env:RUNNER_TEMP ('task164-task4-packaged-review-' + [guid]::NewGuid().ToString('N'))
$evidenceRoot = if (-not [string]::IsNullOrWhiteSpace($env:TASK164_FINAL_EVIDENCE_DIR)) {
    Join-Path $env:TASK164_FINAL_EVIDENCE_DIR 'task4-packaged-review'
} else {
    Join-Path $env:RUNNER_TEMP ('task164-task4-packaged-review-evidence-' + [guid]::NewGuid().ToString('N'))
}
New-Item -ItemType Directory -Force $fixture,$evidenceRoot | Out-Null
Expand-Archive -Path $installZip -DestinationPath $fixture -Force

foreach ($required in @(
    '.code-harness/bin/codea-dcep-tools.exe',
    '.code-harness/bootstrap.md',
    '.code-harness/AGENTS.md',
    '.code-harness/agents/orchestrator.md',
    '.code-harness/contracts/reviewer-host-contract.md',
    '.opencode/agents/reviewer.md',
    '.opencode/commands/harness-review-reviewer.md',
    '.opencode/tools/codea-reviewer-submit.ts'
)) {
    if (-not (Test-Path (Join-Path $fixture $required) -PathType Leaf)) { throw "packaged Task 4 resource missing: $required" }
}

Write-Utf8NoBom (Join-Path $fixture '.gitignore') ".code-harness/`n.opencode/`nopencode.json`n"
$sourcePath = Join-Path $fixture 'src/main/resources/application.yml'
New-Item -ItemType Directory -Force (Split-Path -Parent $sourcePath) | Out-Null
Write-Utf8NoBom $sourcePath "task4Review: false`n"
Push-Location $fixture
try {
    Invoke-Git init -b develop
    Invoke-Git config user.email 'task164-task4@example.test'
    Invoke-Git config user.name 'Task164 Task4 Packaged E2E'
    Invoke-Git config core.autocrlf false
    Invoke-Git add .gitignore src
    Invoke-Git commit -m 'Task 4 packaged review baseline'
}
finally { Pop-Location }
Write-Utf8NoBom $sourcePath "task4Review: true`n"

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
    instructions = @(
        '.code-harness/bootstrap.md',
        '.code-harness/AGENTS.md',
        '.code-harness/agents/orchestrator.md'
    )
    provider = @{
        'task4-local' = @{
            npm = '@ai-sdk/openai-compatible'
            name = 'Task 4 Local Deterministic'
            options = @{ baseURL = "http://127.0.0.1:$port/v1"; apiKey = 'task164-local' }
            models = @{ 'task4' = @{ name = 'Task 4 Deterministic'; limit = @{ context = 200000; output = 4096 } } }
        }
    }
    permission = @{
        '*' = 'deny'
        read = 'allow'
        edit = 'allow'
        bash = 'allow'
        task = 'allow'
    }
} | ConvertTo-Json -Depth 20
Write-Utf8NoBom (Join-Path $fixture 'opencode.json') $config

$serverProcess = Start-Process -FilePath python -ArgumentList @($modelServer,'--port',"$port",'--log',$modelLog,'--scenario','success') -PassThru -WindowStyle Hidden
$passed = $false
try {
    $healthy = $false
    for ($i = 0; $i -lt 60; $i++) {
        try {
            $health = Invoke-RestMethod -Uri "http://127.0.0.1:$port/health" -TimeoutSec 1
            if ($health.status -eq 'ok' -and $health.scenario -eq 'success') { $healthy = $true; break }
        }
        catch { Start-Sleep -Milliseconds 100 }
    }
    if (-not $healthy) { throw 'Task 4 deterministic model provider did not start' }

    Push-Location $fixture
    try {
        $ErrorActionPreference = 'Continue'
        # Two positional tokens preserve literal user input "harness review".
        $raw = (& opencode run --format json --auto --model task4-local/task4 harness review 2>&1 | Out-String)
        $exit = $LASTEXITCODE
        $ErrorActionPreference = 'Stop'
    }
    finally { Pop-Location }
    Write-Utf8NoBom $transcript $raw
    if ($exit -ne 0) { throw "Task 4 literal harness review failed exit=$exit`n$raw" }
    if ($raw.Contains('TASK4_STAGE_')) { throw 'PROMPT_ONLY_PROGRESS_NOT_AUTHORITY: private TASK4_STAGE markers leaked into product transcript' }

    $runDirs = @(Get-ChildItem (Join-Path $fixture '.code-harness/runs') -Directory)
    if ($runDirs.Count -ne 1) {
        $runNames = @($runDirs | ForEach-Object { [string]$_.Name })
        $modelText = ''
        if (Test-Path $modelLog -PathType Leaf) { $modelText = Get-Content -Raw $modelLog }
        throw "Task 4 must create exactly one fresh review run; count=$($runDirs.Count); found=$($runNames -join ',')`nTRANSCRIPT:`n$raw`nMODEL_LOG:`n$modelText"
    }
    $runId = $runDirs[0].Name
    if ($runId -notmatch '^review-[0-9a-f]+$') { throw "unexpected review run id: $runId" }
    $runRoot = $runDirs[0].FullName

    foreach ($artifact in @(
        'runtime/review-progress.json',
        'analysis/change-set.json',
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
        if (-not (Test-Path (Join-Path $runRoot $artifact) -PathType Leaf)) { throw "Task 4 full review missing artifact: $artifact" }
    }

    $progress = Get-Content -Raw (Join-Path $runRoot 'runtime/review-progress.json') | ConvertFrom-Json
    if ([string]$progress.runId -ne $runId -or [string]$progress.status -ne 'SUCCEEDED' -or [string]$progress.terminalStage -ne 'REPORT') {
        throw 'Task 4 Runtime progress is not terminal SUCCEEDED/REPORT'
    }
    $stages = @($progress.stages)
    if ($stages.Count -ne 8 -or @($stages | Where-Object { $_.status -ne 'SUCCEEDED' }).Count -ne 0) {
        throw 'Task 4 Runtime progress does not contain eight succeeded stages'
    }
    $events = @($progress.events)
    for ($i = 0; $i -lt $events.Count; $i++) {
        if ([int]$events[$i].sequence -ne ($i + 1)) { throw "Runtime event sequence is not monotonic at index $i" }
    }
    $expectedPass = @(
        '[1/8] REVIEW_BEGIN PASS',
        '[2/8] SNAPSHOT PASS',
        '[3/8] CHANGE_ANALYSIS PASS',
        '[4/8] CERTIFICATION PASS',
        '[5/8] REVIEW_PLANNING PASS',
        '[6/8] REVIEW_EXECUTION PASS',
        '[7/8] FINDING_CERTIFICATION PASS',
        '[8/8] REPORT PASS'
    )
    foreach ($display in $expectedPass) {
        if (@($events | Where-Object { [string]$_.display -eq $display }).Count -ne 1) {
            throw "Runtime progress missing unique event: $display"
        }
        if (-not $raw.Contains($display)) {
            throw "OpenCode transcript did not render Runtime events[].display: $display"
        }
    }
    Assert-InOrder $raw $expectedPass 'OpenCode Runtime progress transcript'
    Write-Output "TASK164_TASK4_RUNTIME_PROGRESS_TERMINAL PASS runId=$runId stages=8 events=$($events.Count)"
    Write-Output 'OPENCODE_RUNTIME_PROGRESS_RENDERED PASS'
    Write-Output 'OPENCODE_RUNTIME_PROGRESS_1_TO_8 PASS'

    $modelEntries = @(
        foreach ($line in @(Get-Content $modelLog)) {
            if (-not [string]::IsNullOrWhiteSpace($line)) { $line | ConvertFrom-Json }
        }
    )
    $toolResponses = @($modelEntries | Where-Object { $_.responseType -eq 'tool' })
    $bashCalls = @($toolResponses | Where-Object { $_.tool -eq 'bash' })
    $writeCalls = @($toolResponses | Where-Object { $_.tool -eq 'write' })
    $taskCalls = @($toolResponses | Where-Object { $_.tool -eq 'task' })
    if ($bashCalls.Count -lt 8) { throw 'product E2E did not exercise real Runtime command flow' }
    $forbiddenShell = '(?i)(powershell|pwsh|new-item|writealltext|convertfrom-json|out-string|\||>|<|;|&&|\$\(|`)' 
    $allowedRuntime = '^\.code-harness/bin/codea-dcep-tools\.exe (review begin|review progress --run-id review-[0-9a-f]+|analysis snapshot --input \.code-harness/runs/review-[0-9a-f]+/requests/change-set-request\.json|analysis certify --input \.code-harness/runs/review-[0-9a-f]+/requests/analysis-certify-request\.json|review options --input \.code-harness/runs/review-[0-9a-f]+/requests/review-options-request\.json|review select --input \.code-harness/runs/review-[0-9a-f]+/requests/review-selection-request\.json|review units --run-id review-[0-9a-f]+|review dispatch --run-id review-[0-9a-f]+|review certify-findings --input \.code-harness/runs/review-[0-9a-f]+/requests/finding-certify-request\.json|report review --input \.code-harness/runs/review-[0-9a-f]+/requests/review-report\.json)$'
    foreach ($entry in $bashCalls) {
        $command = [string]$entry.arguments.command
        if ($command -match $forbiddenShell) { throw "Harness-contract product E2E used forbidden shell orchestration: $command" }
        if ($command -notmatch $allowedRuntime) { throw "Harness-contract product E2E used non-allowlisted Runtime command: $command" }
    }
    foreach ($entry in $writeCalls) {
        $path = ([string]$entry.arguments.filePath).Replace('\','/')
        if ($path -notmatch "^\.code-harness/runs/$([regex]::Escape($runId))/requests/[^/]+\.json$") {
            throw "Main Agent write escaped same-run requests authority: $path"
        }
        if ($path -match '/analysis/|/review\.md$|\.code-harness/chains/') {
            throw "Main Agent wrote Runtime-owned artifact: $path"
        }
    }
    if ($taskCalls.Count -ne 2) { throw "expected exactly two Reviewer delegations, got $($taskCalls.Count)" }
    foreach ($entry in $taskCalls) {
        if ([string]$entry.arguments.subagent_type -ne 'reviewer' -or [string]$entry.arguments.command -ne 'harness-review-reviewer') {
            throw 'semantic phase was not delegated through the formal Reviewer command'
        }
    }
    $progressSourceEntries = @($toolResponses | Where-Object { $_.runtimeProgressSource -eq $true -and -not [string]::IsNullOrWhiteSpace([string]$_.content) })
    if ($progressSourceEntries.Count -lt 4) { throw 'model log does not prove progress text was derived from Runtime progress results' }
    foreach ($display in $expectedPass) {
        $sourceCount = @($progressSourceEntries | Where-Object { ([string]$_.content).Contains($display) }).Count
        $finalTextCount = @($modelEntries | Where-Object { $_.responseType -eq 'text' -and ([string]$_.content).Contains($display) }).Count
        if (($sourceCount + $finalTextCount) -lt 1) { throw "Runtime-derived progress source missing for transcript display: $display" }
    }
    Write-Output 'PROMPT_ONLY_PROGRESS_NOT_AUTHORITY PASS'

    $report = Get-Content -Raw (Join-Path $runRoot 'review.md')
    if ($report -notmatch [regex]::Escape('| 评审结果 | ✅ 通过 |')) { throw "Task 4 canonical review.md is not PASSED`n$report" }
    Write-Output "TASK164_TASK4_REVIEW_MD PASS runId=$runId"

    $changeReceipt = Get-Content -Raw (Join-Path $runRoot 'requests/change-analysis-reviewer-authority.json') | ConvertFrom-Json
    $findingReceipt = Get-Content -Raw (Join-Path $runRoot 'requests/finding-reviewer-authority.json') | ConvertFrom-Json
    if ($changeReceipt.agent -ne 'reviewer' -or $changeReceipt.proposalKind -ne 'change-analysis' -or [string]::IsNullOrWhiteSpace([string]$changeReceipt.sessionId)) { throw 'Task 4 change-analysis Reviewer authority receipt invalid' }
    if ($findingReceipt.agent -ne 'reviewer' -or $findingReceipt.proposalKind -ne 'findings' -or [string]::IsNullOrWhiteSpace([string]$findingReceipt.sessionId)) { throw 'Task 4 finding Reviewer authority receipt invalid' }
    if ([string]$changeReceipt.sessionId -eq [string]$findingReceipt.sessionId) { throw 'Task 4 semantic phases reused the same Reviewer child session' }

    $rootSessionMatch = [regex]::Match($raw, '"sessionID"\s*:\s*"(?<id>ses_[^"]+)"')
    if (-not $rootSessionMatch.Success) { throw 'Task 4 OpenCode transcript did not expose root sessionID' }
    $rootId = $rootSessionMatch.Groups['id'].Value
    Push-Location $fixture
    try {
        $rootExport = Invoke-OpenCodeExport $rootId
        $changeExport = Invoke-OpenCodeExport ([string]$changeReceipt.sessionId)
        $findingExport = Invoke-OpenCodeExport ([string]$findingReceipt.sessionId)
    }
    finally { Pop-Location }
    Write-Utf8NoBom (Join-Path $evidenceRoot 'root-session.json') $rootExport
    Write-Utf8NoBom (Join-Path $evidenceRoot 'change-reviewer-session.json') $changeExport
    Write-Utf8NoBom (Join-Path $evidenceRoot 'finding-reviewer-session.json') $findingExport
    $rootSession = $rootExport | ConvertFrom-Json
    $rootUsers = @(
        foreach ($message in @($rootSession.messages | Where-Object { $_.info.role -eq 'user' })) {
            foreach ($part in @($message.parts | Where-Object { $_.type -eq 'text' })) { [string]$part.text }
        }
    )
    if ([string]$rootSession.info.id -ne $rootId -or $rootUsers -notcontains 'harness review') { throw 'exported root session does not prove literal user prompt harness review' }
    foreach ($entry in @(
        [pscustomobject]@{Name='change-analysis'; Session=($changeExport | ConvertFrom-Json); Id=[string]$changeReceipt.sessionId},
        [pscustomobject]@{Name='findings'; Session=($findingExport | ConvertFrom-Json); Id=[string]$findingReceipt.sessionId}
    )) {
        $reviewerMessages = @($entry.Session.messages | Where-Object { [string]$_.info.agent -eq 'reviewer' })
        if ([string]$entry.Session.info.id -ne $entry.Id -or [string]$entry.Session.info.parentID -ne $rootId -or $reviewerMessages.Count -eq 0) {
            throw "exported $($entry.Name) Reviewer child lacks independent Reviewer/root-parent identity"
        }
    }
    Write-Output "TASK164_TASK4_INDEPENDENT_REVIEWER_BOTH_PHASES PASS rootSessionId=$rootId changeReviewerSessionId=$($changeReceipt.sessionId) findingReviewerSessionId=$($findingReceipt.sessionId)"

    Copy-Item (Join-Path $runRoot 'runtime/review-progress.json') (Join-Path $evidenceRoot 'review-progress.json') -Force
    Copy-Item (Join-Path $runRoot 'review.md') (Join-Path $evidenceRoot 'review.md') -Force
    Copy-Item (Join-Path $runRoot 'requests/change-analysis-reviewer-authority.json') (Join-Path $evidenceRoot 'change-analysis-reviewer-authority.json') -Force
    Copy-Item (Join-Path $runRoot 'requests/finding-reviewer-authority.json') (Join-Path $evidenceRoot 'finding-reviewer-authority.json') -Force

    Write-Output "TASK164_TASK4_PACKAGED_PLAIN_REVIEW_8_OF_8 PASS runId=$runId"
    Write-Output 'TASK164_TASK4_GATE_B PASS'
    $passed = $true
}
finally {
    if ($serverProcess -and -not $serverProcess.HasExited) { Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue }
    if ($passed) { Remove-Item $fixture -Recurse -Force -ErrorAction SilentlyContinue }
    else { Write-Warning "Task 4 packaged review failure fixture retained at $fixture; evidence=$evidenceRoot" }
}
