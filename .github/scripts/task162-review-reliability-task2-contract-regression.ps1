$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$agents = Get-Content -Raw '.code-harness/AGENTS.md'
$tools = Get-Content -Raw '.code-harness/tools/README.md'
$orchestrator = Get-Content -Raw '.code-harness/agents/orchestrator.md'

foreach ($item in @(
    @{ Name = 'AGENTS'; Text = $agents },
    @{ Name = 'TOOLS'; Text = $tools },
    @{ Name = 'ORCHESTRATOR'; Text = $orchestrator }
)) {
    if ($item.Text -notmatch [regex]::Escape('codea-dcep-tools.exe review begin')) {
        throw "Task 2 $($item.Name) does not expose review begin"
    }
}

foreach ($required in @(
    '每一次新的顶层 `harness review`',
    '`review begin` → fresh runId → `analysis snapshot`',
    '上一轮 runId',
    '上一轮 Snapshot',
    '上一轮 0 Change',
    '对新 invocation 不具备 Authority',
    '`same-run` 只约束单次 Review invocation 内部'
)) {
    if ($orchestrator -notmatch [regex]::Escape($required)) {
        throw "Task 2 Orchestrator fresh lifecycle contract missing: $required"
    }
}

if ($orchestrator -notmatch 'analysis snapshot.*唯一.*Git.*ChangeSet.*Authority') {
    throw 'Task 2 must preserve analysis snapshot as the sole Git ChangeSet Authority'
}

Write-Output 'TASK162_REVIEW_RELIABILITY_TASK2_REVIEW_BEGIN_CONTRACT PASS'
Write-Output 'TASK162_REVIEW_RELIABILITY_TASK2_FRESH_INVOCATION_CONTRACT PASS'
Write-Output 'TASK162_REVIEW_RELIABILITY_TASK2_SNAPSHOT_AUTHORITY_PRESERVED PASS'
