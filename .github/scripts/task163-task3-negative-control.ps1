$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$e2e = Join-Path $PSScriptRoot 'task163-task3-real-multi-chain-same-session-e2e.ps1'
if (!(Test-Path $e2e -PathType Leaf)) { throw "Task 3 E2E missing: $e2e" }

# Task 2 is the accepted baseline immediately before Task 3. Restoring these four
# active Agent contracts creates the negative control: same Runtime, same mock model,
# same fixture, same exact `harness review`, but no Task 3 USER_SELECTION hard-stop contract.
$task2Base = '7866d1402b0016383a88ce49084ab9addda5c46e'
$contracts = @(
    '.code-harness/AGENTS.md',
    '.code-harness/agents/orchestrator.md',
    '.code-harness/agents/reviewer.md',
    '.code-harness/skills/review-code/SKILL.md'
)

try {
    foreach ($path in $contracts) {
        $spec = "${task2Base}:$path"
        $content = (& git -C $repoRoot show $spec 2>&1 | Out-String)
        if ($LASTEXITCODE -ne 0) { throw "negative-control git show failed for $spec`n$content" }
        [System.IO.File]::WriteAllText(
            (Join-Path $repoRoot $path),
            $content,
            [System.Text.UTF8Encoding]::new($false)
        )
    }

    $ErrorActionPreference = 'Continue'
    $output = (& pwsh -NoProfile -File $e2e 2>&1 | Out-String)
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = 'Stop'
}
finally {
    & git -C $repoRoot checkout -- @contracts | Out-Null
    if ($LASTEXITCODE -ne 0) { throw 'failed to restore Task 3 active contracts after negative control' }
}

if ($exitCode -eq 0) {
    throw "TASK163_NEGATIVE_CONTROL_INVALID: same E2E still GREEN without Task 3 hard-stop contract. The mock/test is implementing the stop itself.`n$output"
}
if ($output -notmatch 'TASK163_CONTRACT_GATE_MISSING: Turn 1 illegally produced review select request') {
    throw "TASK163_NEGATIVE_CONTROL_INVALID: E2E failed without Task 3 contract, but not because it crossed USER_SELECTION in the same Assistant Turn.`n$output"
}

Write-Output 'TASK163_TASK3_NEGATIVE_CONTROL_RED PASS'
Write-Output 'TASK163_TASK3_CONTRACT_DRIVEN_HARD_STOP PROVEN'
