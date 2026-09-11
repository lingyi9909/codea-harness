$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$installZip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-install.zip'
$modelServer = Join-Path $PSScriptRoot 'task164-release-blocker-task2-plain-review-server.py'
$evidenceRoot = Join-Path $env:RUNNER_TEMP 'task164-task2-entry-evidence'
New-Item -ItemType Directory -Force $evidenceRoot | Out-Null
foreach ($required in @($installZip, $modelServer)) {
    if (-not (Test-Path $required -PathType Leaf)) { throw "Task 2 entry E2E missing required file: $required" }
}
foreach ($command in @('opencode', 'python', 'git')) {
    if (-not (Get-Command $command -ErrorAction SilentlyContinue)) { throw "Task 2 entry E2E requires $command on PATH" }
}

function Write-Utf8NoBom([string]$Path, [string]$Content) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force $parent | Out-Null }
    [IO.File]::WriteAllText($Path, $Content, [Text.UTF8Encoding]::new($false))
}

function Invoke-Git([Parameter(ValueFromRemainingArguments = $true)][string[]]$Arguments) {
    & git @Arguments | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "git $($Arguments -join ' ') failed with exit code $LASTEXITCODE" }
}

function Assert-InOrder([string]$Text, [string[]]$Markers, [string]$Name) {
    $previous = -1
    foreach ($marker in $Markers) {
        $index = $Text.IndexOf($marker, [StringComparison]::Ordinal)
        if ($index -lt 0) { throw "$Name missing ordered marker: $marker" }
        if ($index -le $previous) { throw "$Name marker is out of order: $marker" }
        $previous = $index
    }
}

function Invoke-OpenCodeExport([string]$SessionId) {
    $stderrPath = Join-Path $env:RUNNER_TEMP ("task164-opencode-export-$SessionId-" + [guid]::NewGuid().ToString('N') + '.stderr.log')
    try {
        $stdout = (& opencode export $SessionId 2> $stderrPath | Out-String)
        $exit = $LASTEXITCODE
        if ($exit -ne 0) {
            $stderr = if (Test-Path $stderrPath) { Get-Content -Raw $stderrPath } else { '' }
            throw "opencode export $SessionId failed exit=$exit`n$stderr"
        }
        return $stdout
    }
    finally { Remove-Item $stderrPath -Force -ErrorAction SilentlyContinue }
}

function New-Fixture([ValidateSet('positive','disabled')][string]$Scenario) {
    $fixture = Join-Path $env:RUNNER_TEMP ("task164-task2-entry-$Scenario-" + [guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Force $fixture | Out-Null
    Expand-Archive -Path $installZip -DestinationPath $fixture -Force

    foreach ($required in @(
        '.code-harness/bin/codea-dcep-tools.exe',
        '.code-harness/agents/orchestrator.md',
        '.code-harness/contracts/reviewer-host-contract.md',
        '.opencode/agents/reviewer.md',
        '.opencode/commands/harness-review-reviewer.md',
        '.opencode/tools/codea-reviewer-submit.ts'
    )) {
        if (-not (Test-Path (Join-Path $fixture $required) -PathType Leaf)) { throw "packaged Task 2 entry resource missing: $required" }
    }
    if ($Scenario -eq 'disabled') {
        Remove-Item (Join-Path $fixture '.opencode/agents/reviewer.md') -Force
    }

    Write-Utf8NoBom (Join-Path $fixture '.gitignore') ".code-harness/`n.opencode/`nopencode.json`n"
    Write-Utf8NoBom (Join-Path $fixture 'src/main/resources/application.yml') "entryReview: false`n"
    Push-Location $fixture
    try {
        Invoke-Git init -b develop
        Invoke-Git config user.email 'task164-entry-e2e@example.test'
        Invoke-Git config user.name 'Task164 Entry E2E'
        Invoke-Git config core.autocrlf false
        Invoke-Git add .gitignore src
        Invoke-Git commit -m 'Task 2 packaged entry E2E baseline'
    }
    finally { Pop-Location }
    Write-Utf8NoBom (Join-Path $fixture 'src/main/resources/application.yml') "entryReview: true`n"
    return $fixture
}

function Invoke-EntryScenario([ValidateSet('positive','disabled')][string]$Scenario) {
    $fixture = New-Fixture $Scenario
    $serverProcess = $null
    $scenarioPassed = $false
    $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback, 0)
    $listener.Start()
    $port = ([Net.IPEndPoint]$listener.LocalEndpoint).Port
    $listener.Stop()
    $scenarioEvidence = Join-Path $evidenceRoot $Scenario
    New-Item -ItemType Directory -Force $scenarioEvidence | Out-Null
    $modelLog = Join-Path $scenarioEvidence 'model.jsonl'
    $transcript = Join-Path $scenarioEvidence 'opencode-run.jsonl'

    try {
        $config = @{
            '$schema' = 'https://opencode.ai/config.json'
            model = 'task2-entry-local/task2-entry'
            small_model = 'task2-entry-local/task2-entry'
            shell = 'pwsh'
            instructions = @(
                '.code-harness/bootstrap.md',
                '.code-harness/AGENTS.md',
                '.code-harness/agents/orchestrator.md'
            )
            provider = @{
                'task2-entry-local' = @{
                    npm = '@ai-sdk/openai-compatible'
                    name = 'Task 2 Entry Local Deterministic'
                    options = @{ baseURL = "http://127.0.0.1:$port/v1"; apiKey = 'task164-local' }
                    models = @{ 'task2-entry' = @{ name = 'Task 2 Entry Deterministic'; limit = @{ context = 200000; output = 4096 } } }
                }
            }
            permission = @{ read = 'allow'; edit = 'allow'; bash = 'allow'; webfetch = 'deny'; websearch = 'deny' }
        }
        Write-Utf8NoBom (Join-Path $fixture 'opencode.json') ($config | ConvertTo-Json -Depth 20)

        $runsRoot = Join-Path $fixture '.code-harness/runs'
        if (Test-Path $runsRoot) {
            $preexistingRuns = @(Get-ChildItem $runsRoot -Directory -ErrorAction SilentlyContinue)
            if ($preexistingRuns.Count -ne 0) { throw "package unexpectedly contained pre-run review sessions: $($preexistingRuns.Name -join ',')" }
        }

        $serverProcess = Start-Process -FilePath python -ArgumentList @($modelServer, '--port', "$port", '--log', $modelLog, '--scenario', $Scenario) -PassThru -WindowStyle Hidden
        $healthy = $false
        for ($i = 0; $i -lt 50; $i++) {
            try {
                $health = Invoke-RestMethod -Uri "http://127.0.0.1:$port/health" -TimeoutSec 1
                if ($health.status -eq 'ok') { $healthy = $true; break }
            }
            catch { Start-Sleep -Milliseconds 100 }
        }
        if (-not $healthy) { throw "Task 2 entry $Scenario provider did not start" }

        Push-Location $fixture
        try {
            $ErrorActionPreference = 'Continue'
            # OpenCode quotes each positional argument that already contains a
            # space. Two tokens preserve the user's exact top-level text.
            $raw = (& opencode run --format json --auto --model task2-entry-local/task2-entry harness review 2>&1 | Out-String)
            $exit = $LASTEXITCODE
            $ErrorActionPreference = 'Stop'
        }
        finally { Pop-Location }
        Write-Utf8NoBom $transcript $raw
        if ($exit -ne 0) { throw "top-level OpenCode $Scenario harness review failed exit=${exit}:`n$raw" }

        $modelRequests = @(
            if (Test-Path $modelLog) {
                foreach ($line in @(Get-Content $modelLog)) {
                    if (-not [string]::IsNullOrWhiteSpace($line)) { $line | ConvertFrom-Json }
                }
            }
        )
        $modelUserTexts = @(
            foreach ($request in $modelRequests) {
                foreach ($message in @($request.messages | Where-Object { $_.role -eq 'user' })) {
                    if ($message.content -is [string]) { [string]$message.content; continue }
                    foreach ($block in @($message.content)) {
                        if ($block -is [string]) { [string]$block }
                        elseif (-not [string]::IsNullOrWhiteSpace([string]$block.text)) { [string]$block.text }
                    }
                }
            }
        )
        if ($modelUserTexts -notcontains 'harness review') {
            $compactUsers = @($modelUserTexts | Select-Object -First 12 | ForEach-Object { if ($_.Length -gt 200) { $_.Substring(0, 200) + '...' } else { $_ } })
            $compactTools = @($modelRequests | ForEach-Object { @($_.toolNames) } | Sort-Object -Unique)
            throw "provider did not receive literal top-level user prompt harness review; requests=$($modelRequests.Count); userTexts=$($compactUsers -join ' || '); tools=$($compactTools -join ',')"
        }
        $runMatch = [regex]::Match($raw, 'TASK2_ENTRY_REVIEW_BEGIN runId=(?<id>review-[0-9a-f]+)')
        if (-not $runMatch.Success) { throw "top-level entry did not expose Runtime fresh runId:`n$raw" }
        $runId = $runMatch.Groups['id'].Value
        $runRoot = Join-Path $fixture ".code-harness/runs/$runId"
        $rootSessionMatch = [regex]::Match($raw, '"sessionID"\s*:\s*"(?<id>ses_[^"]+)"')
        if (-not $rootSessionMatch.Success) { throw 'top-level OpenCode transcript did not expose its root sessionID' }
        Push-Location $fixture
        try { $topLevelExport = Invoke-OpenCodeExport $rootSessionMatch.Groups['id'].Value }
        finally { Pop-Location }
        Write-Utf8NoBom (Join-Path $scenarioEvidence 'root-session.json') $topLevelExport
        $topLevelSession = $topLevelExport | ConvertFrom-Json
        $rootUserTexts = @(
            foreach ($message in @($topLevelSession.messages | Where-Object { $_.info.role -eq 'user' })) {
                foreach ($part in @($message.parts | Where-Object { $_.type -eq 'text' })) { [string]$part.text }
            }
        )
        if ([string]$topLevelSession.info.id -ne $rootSessionMatch.Groups['id'].Value -or $rootUserTexts -notcontains 'harness review') {
            throw 'exported root session does not prove the literal user prompt harness review'
        }
        $runDirectories = @(Get-ChildItem $runsRoot -Directory)
        if ($runDirectories.Count -ne 1 -or $runDirectories[0].Name -ne $runId) {
            throw "top-level entry did not create exactly one fresh Runtime run: $($runDirectories.Name -join ',')"
        }

        if ($Scenario -eq 'positive') {
            Assert-InOrder $raw @(
                'TASK2_ENTRY_REVIEW_BEGIN',
                'TASK2_ENTRY_SNAPSHOT',
                '"subagent_type":"reviewer"',
                'TASK2_ENTRY_REVIEWER_CHILD_COMPLETE',
                'TASK2_ENTRY_PROPOSAL',
                'TASK2_ENTRY_CERTIFY',
                'TASK2_ENTRY_REVIEW_OPTIONS'
            ) 'Task 2 top-level positive chain'
            foreach ($artifact in @(
                'analysis/change-set.json',
                'requests/change-analysis-proposal.json',
                'requests/change-analysis-reviewer-authority.json',
                'analysis/change-analysis.json',
                'analysis/entrypoint-inventory.json',
                'analysis/change-analysis.cert.json',
                'analysis/review-options.json'
            )) {
                if (-not (Test-Path (Join-Path $runRoot $artifact) -PathType Leaf)) { throw "top-level positive chain missing $artifact" }
            }
            $receipt = Get-Content -Raw (Join-Path $runRoot 'requests/change-analysis-reviewer-authority.json') | ConvertFrom-Json
            if ($receipt.agent -ne 'reviewer' -or $receipt.proposalKind -ne 'change-analysis' -or [string]::IsNullOrWhiteSpace([string]$receipt.sessionId)) {
                throw "top-level Reviewer authority receipt is invalid"
            }
            if ($raw -notmatch [regex]::Escape([string]$receipt.sessionId)) { throw 'Reviewer receipt session is absent from real OpenCode transcript' }
            $parent = [regex]::Match($raw, '"parentSessionId"\s*:\s*"(?<id>ses_[^"]+)"')
            if (-not $parent.Success -or $parent.Groups['id'].Value -eq [string]$receipt.sessionId) { throw 'Reviewer child session is not independent from its parent' }
            if ($parent.Groups['id'].Value -ne $rootSessionMatch.Groups['id'].Value) { throw 'Reviewer parentSessionId does not match the exported root session' }
            Push-Location $fixture
            try {
                $childExport = Invoke-OpenCodeExport ([string]$receipt.sessionId)
            }
            finally { Pop-Location }
            Write-Utf8NoBom (Join-Path $scenarioEvidence 'reviewer-child-session.json') $childExport
            $childSession = $childExport | ConvertFrom-Json
            $reviewerMessages = @($childSession.messages | Where-Object { [string]$_.info.agent -eq 'reviewer' })
            if ([string]$childSession.info.id -ne [string]$receipt.sessionId -or [string]$childSession.info.parentID -ne $parent.Groups['id'].Value -or $reviewerMessages.Count -eq 0) {
                throw 'exported Reviewer child does not identify its root parent and Reviewer role'
            }
            $certificate = Get-Content -Raw (Join-Path $runRoot 'analysis/change-analysis.cert.json') | ConvertFrom-Json
            $options = Get-Content -Raw (Join-Path $runRoot 'analysis/review-options.json') | ConvertFrom-Json
            if ([string]$certificate.runId -ne $runId -or [string]$options.runId -ne $runId -or [string]::IsNullOrWhiteSpace([string]$options.decision)) {
                throw 'Runtime certificate or ReviewOptions is not bound to the fresh entry run'
            }
            Copy-Item (Join-Path $runRoot 'requests/change-analysis-reviewer-authority.json') (Join-Path $scenarioEvidence 'change-analysis-reviewer-authority.json') -Force
            Copy-Item (Join-Path $runRoot 'analysis/change-analysis.cert.json') (Join-Path $scenarioEvidence 'change-analysis.cert.json') -Force
            Copy-Item (Join-Path $runRoot 'analysis/review-options.json') (Join-Path $scenarioEvidence 'review-options.json') -Force
            Write-Output "TASK164_TASK2_TOP_LEVEL_REVIEW_CHAIN PASS runId=$runId rootSessionId=$($parent.Groups['id'].Value) reviewerChildSessionId=$($receipt.sessionId) analysisSha256=$($certificate.analysisSha256) optionsHash=$($options.optionsHash) decision=$($options.decision) ordered=review_begin>snapshot>reviewer_child>proposal>analysis_certify>review_options"
        }
        else {
            Assert-InOrder $raw @('TASK2_ENTRY_REVIEW_BEGIN','TASK2_ENTRY_SNAPSHOT','TASK2_ENTRY_RUNTIME_HARD_STOP_BEGIN') 'Task 2 top-level disabled lifecycle'
            $hardStopIndex = $raw.IndexOf('TASK2_ENTRY_RUNTIME_HARD_STOP_BEGIN', [StringComparison]::Ordinal)
            $hardStopEvidence = $raw.Substring($hardStopIndex)
            Assert-InOrder $hardStopEvidence @('REVIEWER_UNAVAILABLE','MANUAL_ACTION_REQUIRED','HARD STOP') 'Task 2 top-level disabled Runtime hard stop'
            foreach ($forbidden in @(
                'requests/change-analysis-proposal.json',
                'requests/change-analysis-reviewer-authority.json',
                'analysis/change-analysis.json',
                'analysis/entrypoint-inventory.json',
                'analysis/change-analysis.cert.json',
                'analysis/review-options.json',
                'analysis/review-scope.json',
                'analysis/review-units.json',
                'analysis/rule-dispatch.json',
                'analysis/certified-findings.json',
                'analysis/certified-findings.cert.json',
                'review.md'
            )) {
                if (Test-Path (Join-Path $runRoot $forbidden)) { throw "Reviewer-disabled entry published downstream authority artifact: $forbidden" }
            }
            if ($raw -match '"subagent_type":"reviewer"') { throw 'Reviewer-disabled entry spawned a Reviewer child' }
            Write-Output "TASK164_TASK2_TOP_LEVEL_DISABLED_HARD_STOP PASS runId=$runId ordered=review_begin>snapshot>REVIEWER_UNAVAILABLE>MANUAL_ACTION_REQUIRED>HARD_STOP downstreamAuthorityArtifacts=0"
        }
        $scenarioPassed = $true
    }
    finally {
        if ($serverProcess -and -not $serverProcess.HasExited) { Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue }
        if ($scenarioPassed) {
            Remove-Item $fixture -Recurse -Force -ErrorAction SilentlyContinue
        }
        else {
            Write-Warning "Task 2 entry $Scenario evidence retained: fixture=$fixture modelLog=$modelLog transcript=$transcript"
        }
    }
}

Invoke-EntryScenario positive
Invoke-EntryScenario disabled
Write-Output 'gate_task2_entry_e2e PASS'
