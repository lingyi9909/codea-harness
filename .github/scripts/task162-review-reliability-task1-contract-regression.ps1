$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Push-Location $repoRoot
try {
    $requiredSchemas = @(
        '.code-harness/contracts/finding-certify-request.schema.json',
        '.code-harness/contracts/report-review-request.schema.json'
    )
    foreach ($schema in $requiredSchemas) {
        if (-not (Test-Path $schema -PathType Leaf)) { throw "Task 1 required request schema missing: $schema" }
    }

    $activeContracts = @(
        '.code-harness/AGENTS.md',
        '.code-harness/tools/README.md',
        '.code-harness/agents/orchestrator.md',
        '.code-harness/agents/reviewer.md'
    )
    $requiredCommands = @(
        'codea-dcep-tools.exe review options --input',
        'codea-dcep-tools.exe review select --input',
        'codea-dcep-tools.exe review units --run-id',
        'codea-dcep-tools.exe review dispatch --run-id',
        'codea-dcep-tools.exe review certify-findings --input',
        'codea-dcep-tools.exe report review --input'
    )
    foreach ($contract in $activeContracts) {
        $text = Get-Content -Raw $contract
        foreach ($command in $requiredCommands) {
            if ($text -notmatch [regex]::Escape($command)) {
                throw "Task 1 active contract $contract does not expose formal command: $command"
            }
        }
        foreach ($schema in @('finding-certify-request.schema.json','report-review-request.schema.json')) {
            if ($text -notmatch [regex]::Escape($schema)) {
                throw "Task 1 active contract $contract does not expose request schema: $schema"
            }
        }
    }

    $orchestrator = Get-Content -Raw '.code-harness/agents/orchestrator.md'
    foreach ($required in @(
        'changedFiles=[]',
        'finding-proposals.json',
        'review certify-findings',
        'report review'
    )) {
        if ($orchestrator -notmatch [regex]::Escape($required)) {
            throw "Task 1 Orchestrator zero-change/full-authority rule missing: $required"
        }
    }

    $tools = Get-Content -Raw '.code-harness/tools/README.md'
    if ($tools -notmatch 'findings[^\r\n]*\[\]') {
        throw 'Task 1 Tool Contract must state that Agent-facing formal report request carries findings: [] only'
    }
    if ($tools -match 'review-output\.schema\.json[^\r\n]*(report review|report-review)') {
        throw 'Task 1 Tool Contract still presents review-output.schema.json as report review request contract'
    }

    Write-Output 'TASK162_REVIEW_RELIABILITY_TASK1_ACTIVE_COMMAND_CONTRACT PASS'
    Write-Output 'TASK162_REVIEW_RELIABILITY_TASK1_REQUEST_SCHEMA_DISCOVERY PASS'
    Write-Output 'TASK162_REVIEW_RELIABILITY_TASK1_ZERO_CHANGE_AUTHORITY_CONTRACT PASS'
} finally {
    Pop-Location
}
