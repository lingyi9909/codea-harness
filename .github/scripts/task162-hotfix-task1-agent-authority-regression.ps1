$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
Push-Location $repoRoot
try {
    $agents = Get-Content '.code-harness/AGENTS.md' -Raw
    $reviewer = Get-Content '.code-harness/agents/reviewer.md' -Raw
    $orchestrator = Get-Content '.code-harness/agents/orchestrator.md' -Raw
    $analyze = Get-Content '.code-harness/skills/analyze-change/SKILL.md' -Raw
    $tools = Get-Content '.code-harness/tools/README.md' -Raw

    $activeDocs = @(
        @('AGENTS.md', $agents),
        @('reviewer.md', $reviewer),
        @('orchestrator.md', $orchestrator),
        @('analyze-change/SKILL.md', $analyze),
        @('tools/README.md', $tools)
    )

    foreach ($pair in $activeDocs) {
        if ($pair[1] -notmatch 'analysis snapshot') {
            throw "Task1 canonical authority missing analysis snapshot in $($pair[0])"
        }
    }

    # Agent-facing snapshot request contract must exactly use Runtime's strict JSON field names.
    $requiredRequestFields = @(
        '"runId": "<runId>"',
        '"baseRef": "<baseRef>"',
        '"includeWorkingTree": true'
    )
    foreach ($pair in $activeDocs) {
        foreach ($field in $requiredRequestFields) {
            if ($pair[1] -notmatch [regex]::Escape($field)) {
                throw "Task1 snapshot request contract missing $field in $($pair[0])"
            }
        }
    }

    # requestedBaseRef is Runtime Snapshot provenance only; these historical request forms are forbidden.
    foreach ($pair in $activeDocs) {
        $text = [string]$pair[1]
        if ($text -match 'runId\s*/\s*requestedBaseRef\s*/\s*includeWorkingTree') {
            throw "Task1 Agent-facing snapshot request still uses requestedBaseRef in $($pair[0])"
        }
        if ($text -match '(?s)Snapshot 请求参数.{0,220}requestedBaseRef.{0,220}includeWorkingTree') {
            throw "Task1 Snapshot request parameter prose still uses requestedBaseRef in $($pair[0])"
        }
        if ($text -match '(?s)请求只提供[:：].{0,160}requestedBaseRef.{0,160}includeWorkingTree') {
            throw "Task1 snapshot request field list still uses requestedBaseRef in $($pair[0])"
        }
        if ($text -match '(?s)请求只携带.{0,160}requestedBaseRef.{0,160}includeWorkingTree') {
            throw "Task1 snapshot request field list still uses requestedBaseRef in $($pair[0])"
        }
    }

    if ($analyze -match '(?m)^\s*-\s+git_diff\s*$') {
        throw 'Task1 analyze-change still declares git_diff as an Agent tool'
    }
    if ($reviewer -match '`git_diff` 产生的完整 Review Change Set') {
        throw 'Task1 Reviewer still treats Agent git_diff as Change Set authority'
    }

    $active = @($agents, $reviewer, $orchestrator, $analyze)
    foreach ($text in $active) {
        if ($text -match 'change-analysis-draft\.json') {
            throw 'Task1 active Agent contract still references deterministic change-analysis-draft.json'
        }
    }

    foreach ($text in @($agents, $reviewer, $orchestrator, $analyze)) {
        if ($text -notmatch 'change-analysis-proposal\.json') {
            throw 'Task1 active Agent contract must name change-analysis-proposal.json'
        }
    }

    if (-not (Test-Path '.code-harness/contracts/change-analysis-proposal.schema.json' -PathType Leaf)) {
        throw 'Task1 semantic proposal schema missing'
    }
    if (-not (Test-Path '.code-harness/contracts/change-set.schema.json' -PathType Leaf)) {
        throw 'Task1 canonical ChangeSet schema missing'
    }

    Write-Output 'TASK162_HOTFIX_TASK1_AGENT_SNAPSHOT_REQUEST_CONTRACT PASS'
    Write-Output 'TASK162_HOTFIX_TASK1_AGENT_AUTHORITY PASS'
}
finally {
    Pop-Location
}
