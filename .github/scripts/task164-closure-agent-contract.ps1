$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$paths = [ordered]@{
    agents = Join-Path $repoRoot '.code-harness/AGENTS.md'
    bootstrap = Join-Path $repoRoot '.code-harness/bootstrap.md'
    orchestrator = Join-Path $repoRoot '.code-harness/agents/orchestrator.md'
    reviewerContract = Join-Path $repoRoot '.code-harness/contracts/reviewer-host-contract.md'
}
$text = @{}
foreach ($entry in $paths.GetEnumerator()) {
    if (-not (Test-Path $entry.Value -PathType Leaf)) { throw "missing contract file: $($entry.Value)" }
    $text[$entry.Key] = Get-Content $entry.Value -Raw
}

$progress = 'codea-dcep-tools.exe review progress --run-id <runId>'
$unavailable = 'codea-dcep-tools.exe review reviewer-unavailable --run-id <runId>'
foreach ($command in @($progress, $unavailable)) {
    if (-not $text.agents.Contains($command)) { throw "AGENTS Runtime allowlist missing: $command" }
}

foreach ($bad in @(
    '| `harness review` | Reviewer |',
    '| `harness review list` | Reviewer（LIST） |',
    '| `harness review <Class>` | Reviewer（TARGETED CLASS） |',
    '| `harness review <Class.method>` | Reviewer（TARGETED METHOD） |'
)) {
    if ($text.orchestrator.Contains($bad)) { throw "orchestrator still assigns entire review flow to Reviewer: $bad" }
}

$requiredTokens = @(
    '1.6.4 Review Host Authority Flow',
    'Main Agent / Orchestrator',
    'independent Reviewer CHANGE_ANALYSIS',
    'Runtime certification',
    'Runtime planning',
    'independent Reviewer FINDINGS',
    'Runtime finding certification',
    'Runtime report',
    'review progress',
    'review reviewer-unavailable'
)
foreach ($name in @('agents','bootstrap','orchestrator','reviewerContract')) {
    foreach ($token in $requiredTokens) {
        if (-not $text[$name].Contains($token)) { throw "$name missing canonical 1.6.4 review authority token: $token" }
    }
}

# Reviewer ownership must remain exactly semantic-proposal-only. These phrases
# deliberately distinguish ownership from mere mentions elsewhere in the docs.
foreach ($name in @('agents','bootstrap','orchestrator','reviewerContract')) {
    if (-not $text[$name].Contains('Reviewer owns only the two semantic proposal phases')) {
        throw "$name does not constrain Reviewer ownership to the two semantic proposal phases"
    }
    if (-not $text[$name].Contains('Main Agent / Orchestrator owns routing, Runtime invocation, Reviewer delegation, Runtime progress rendering, and fail-closed handling')) {
        throw "$name does not define Main Agent / Orchestrator ownership"
    }
}

Write-Output 'HARNESS_164_AGENT_CONTRACT_CONSISTENT PASS'
