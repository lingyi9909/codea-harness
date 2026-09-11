$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$installZip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-install.zip'
if (-not (Test-Path $installZip -PathType Leaf)) { throw "missing candidate install package: $installZip" }
if (-not (Get-Command opencode -ErrorAction SilentlyContinue)) { throw 'pinned OpenCode CLI is required' }

$failures = 0
$entries = @([IO.Compression.ZipFile]::OpenRead($installZip).Entries | ForEach-Object { $_.FullName.Replace('\\','/') })
if ($entries -contains '.opencode/agents/reviewer.md') {
    Write-Output 'gate_reviewer_host_recognition UNEXPECTED_PASS'
} else {
    Write-Output 'gate_reviewer_host_recognition FAIL missing=.opencode/agents/reviewer.md'
    $failures++
}

$fixture = Join-Path $env:RUNNER_TEMP ('task164-task2-red-' + [guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Force $fixture | Out-Null
Expand-Archive -Path $installZip -DestinationPath $fixture -Force
Push-Location $fixture
try {
    $inventory = (& opencode agent list 2>&1 | Out-String)
    if ($LASTEXITCODE -eq 0 -and $inventory -match '(?m)^reviewer\b') {
        Write-Output 'gate_reviewer_reachable UNEXPECTED_PASS'
    } else {
        Write-Output 'gate_reviewer_reachable FAIL reviewer_not_resolvable_by_host'
        $failures++
    }
} finally {
    Pop-Location
}

$orchestrator = Get-Content -Raw (Join-Path $repoRoot '.code-harness/agents/orchestrator.md')
$analysisSkill = Get-Content -Raw (Join-Path $repoRoot '.code-harness/skills/analyze-change/SKILL.md')
$reviewSkill = Get-Content -Raw (Join-Path $repoRoot '.code-harness/skills/review-code/SKILL.md')
$hasUnavailableStop = $orchestrator -match 'REVIEWER_UNAVAILABLE' -and $orchestrator -match 'MANUAL_ACTION_REQUIRED'
$forbidsMainFallback = ($orchestrator + "`n" + $analysisSkill + "`n" + $reviewSkill) -match 'Main Agent.*(不得|禁止).*Reviewer|Reviewer.*(不可用|unavailable).*(STOP|停止)'
if ($hasUnavailableStop -and $forbidsMainFallback) {
    Write-Output 'gate_reviewer_fail_closed UNEXPECTED_PASS'
} else {
    Write-Output 'gate_reviewer_fail_closed FAIL missing_structural_unavailable_stop_or_fallback_ban'
    $failures++
}

if ($failures -eq 0) { throw 'Task 2 RED probe unexpectedly passed; baseline is not RED' }
Write-Output "TASK164_TASK2_EXPECTED_RED PASS failures=$failures"
exit 1
