$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$installZip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-install.zip'
if (-not (Test-Path $installZip -PathType Leaf)) { throw "Task 4 interruption E2E missing candidate install package: $installZip" }
if (-not (Get-Command git -ErrorAction SilentlyContinue)) { throw 'Task 4 interruption E2E requires git' }

$utf8 = [Text.UTF8Encoding]::new($false)
function Write-Utf8NoBom([string]$Path, [string]$Content) {
    $parent = Split-Path -Parent $Path
    if ($parent) { New-Item -ItemType Directory -Force $parent | Out-Null }
    [IO.File]::WriteAllText($Path, $Content, $utf8)
}

$fixture = Join-Path $env:RUNNER_TEMP ('task164-task4-interruption-' + [guid]::NewGuid().ToString('N'))
$evidenceRoot = if (-not [string]::IsNullOrWhiteSpace($env:TASK164_FINAL_EVIDENCE_DIR)) {
    Join-Path $env:TASK164_FINAL_EVIDENCE_DIR 'task4-progress-interruption'
} else {
    Join-Path $env:RUNNER_TEMP ('task164-task4-interruption-evidence-' + [guid]::NewGuid().ToString('N'))
}
New-Item -ItemType Directory -Force $fixture,$evidenceRoot | Out-Null
Expand-Archive -Path $installZip -DestinationPath $fixture -Force
$runtime = Join-Path $fixture '.code-harness/bin/codea-dcep-tools.exe'
if (-not (Test-Path $runtime -PathType Leaf)) { throw "Task 4 interruption E2E missing packaged Runtime: $runtime" }

Write-Utf8NoBom (Join-Path $fixture '.gitignore') ".code-harness/`n"
$sourcePath = Join-Path $fixture 'src/main/resources/application.yml'
New-Item -ItemType Directory -Force (Split-Path -Parent $sourcePath) | Out-Null
Write-Utf8NoBom $sourcePath "interruption: false`n"
& git -C $fixture init -b develop | Out-Null
& git -C $fixture config user.email 'task164-interruption@example.test'
& git -C $fixture config user.name 'Task164 Task4 Interruption E2E'
& git -C $fixture config core.autocrlf false
& git -C $fixture add .gitignore src
& git -C $fixture commit -m 'Task 4 interruption baseline' | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Task 4 interruption E2E could not create git baseline' }
Write-Utf8NoBom $sourcePath "interruption: true`n"

$passed = $false
try {
    Push-Location $fixture
    try {
        $beginRaw = (& $runtime review begin 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "review begin failed`n$beginRaw" }
        $begin = $beginRaw | ConvertFrom-Json
        $runId = [string]$begin.runId
        if ($runId -notmatch '^review-[0-9a-f]+$') { throw "invalid fresh review runId: $runId" }

        $requestRoot = ".code-harness/runs/$runId/requests"
        $requestPath = "$requestRoot/change-set-request.json"
        $request = [ordered]@{ runId=$runId; baseRef='HEAD'; includeWorkingTree=$true }
        Write-Utf8NoBom (Join-Path $fixture $requestPath) ($request | ConvertTo-Json -Compress)
        $snapshotRaw = (& $runtime analysis snapshot --input $requestPath 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "snapshot failed`n$snapshotRaw" }

        $beforeRaw = (& $runtime review progress --run-id $runId 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "progress before interruption failed`n$beforeRaw" }
        $before = $beforeRaw | ConvertFrom-Json
        if ($before.status -ne 'RUNNING' -or $before.currentStage -ne 'CHANGE_ANALYSIS') {
            throw "expected CHANGE_ANALYSIS RUNNING before interruption`n$beforeRaw"
        }
        Write-Output "TASK164_TASK4_INTERRUPTION_ARMED PASS runId=$runId currentStage=$($before.currentStage)"

        $ErrorActionPreference = 'Continue'
        $failureRaw = (& $runtime review reviewer-unavailable --run-id $runId 2>&1 | Out-String)
        $failureExit = $LASTEXITCODE
        $ErrorActionPreference = 'Stop'
        if ($failureExit -eq 0) { throw "reviewer-unavailable unexpectedly succeeded`n$failureRaw" }
        foreach ($marker in @('REVIEWER_UNAVAILABLE','MANUAL_ACTION_REQUIRED','HARD STOP')) {
            if ($failureRaw -notmatch [regex]::Escape($marker)) { throw "interruption output missing $marker`n$failureRaw" }
        }

        $afterRaw = (& $runtime review progress --run-id $runId 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "progress after interruption failed`n$afterRaw" }
        $after = $afterRaw | ConvertFrom-Json
        if ($after.status -ne 'FAILED' -or $after.currentStage -ne 'CHANGE_ANALYSIS' -or $after.failureStage -ne 'CHANGE_ANALYSIS' -or $after.failureCode -ne 'REVIEWER_UNAVAILABLE') {
            throw "Runtime did not attribute failure to CHANGE_ANALYSIS`n$afterRaw"
        }
        $stages = @($after.stages)
        if ($stages.Count -ne 8) { throw "expected 8 Runtime stages, got $($stages.Count)" }
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
        for ($i = 0; $i -lt $expected.Count; $i++) {
            if ([string]$stages[$i].name -ne $expected[$i][0] -or [string]$stages[$i].status -ne $expected[$i][1]) {
                throw "unexpected Runtime stage[$i]: $($stages[$i].name)/$($stages[$i].status)"
            }
        }
        $failureEvents = @($after.events | Where-Object { $_.stage -eq 'CHANGE_ANALYSIS' -and $_.status -eq 'FAILED' -and $_.failureCode -eq 'REVIEWER_UNAVAILABLE' })
        if ($failureEvents.Count -ne 1) { throw 'missing unique CHANGE_ANALYSIS FAILED Runtime event' }

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
            'analysis/certified-findings.json',
            'analysis/certified-findings.cert.json',
            'review.md'
        )) {
            if (Test-Path (Join-Path $fixture ".code-harness/runs/$runId/$forbidden")) {
                throw "downstream artifact exists after CHANGE_ANALYSIS interruption: $forbidden"
            }
        }

        Write-Utf8NoBom (Join-Path $evidenceRoot 'failure-output.txt') $failureRaw
        Write-Utf8NoBom (Join-Path $evidenceRoot 'review-progress.json') $afterRaw
        Write-Output "TASK164_TASK4_INTERRUPTION_CHANGE_ANALYSIS PASS runId=$runId failureCode=$($after.failureCode)"
        Write-Output 'TASK164_TASK4_DOWNSTREAM_BLOCKED PASS certification=BLOCKED planning=BLOCKED execution=BLOCKED findingCertification=BLOCKED report=BLOCKED'
        Write-Output 'TASK164_TASK4_GATE_D PASS'
        $passed = $true
    }
    finally { Pop-Location }
}
finally {
    if ($passed) {
        Remove-Item $fixture -Recurse -Force -ErrorAction SilentlyContinue
    }
    else {
        Write-Warning "Task 4 interruption failure fixture retained at $fixture; evidence=$evidenceRoot"
    }
}
