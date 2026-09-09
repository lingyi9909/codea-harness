$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$failures = [System.Collections.Generic.List[string]]::new()

function Require([bool]$Condition, [string]$Name) {
    if ($Condition) { Write-Output "$Name PASS" } else { $failures.Add($Name) }
}

$upgradeZip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-upgrade.zip'
if (-not (Test-Path $upgradeZip -PathType Leaf)) { throw "missing upgrade package: $upgradeZip" }
Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [IO.Compression.ZipFile]::OpenRead($upgradeZip)
try { $entries = @($zip.Entries | ForEach-Object { $_.FullName.Replace('\','/') }) } finally { $zip.Dispose() }
Require (-not ($entries | Where-Object { $_ -like '.opencode/*' })) 'OFFICIAL_UPGRADE_NO_PRETRANSACTION_HOST_WRITE'
Require ($entries -contains '.code-harness-upgrade/host/.opencode/agents/reviewer.md') 'OFFICIAL_UPGRADE_REVIEWER_HOST_STAGED_SOURCE'
Require ($entries -contains '.code-harness-upgrade/host/.opencode/commands/harness-review-reviewer.md') 'OFFICIAL_UPGRADE_REVIEWER_COMMAND_STAGED_SOURCE'

$upgradeRuntime = Get-Content (Join-Path $repoRoot '.code-harness/tools-runtime/internal/upgrade/upgrade.go') -Raw
Require ($upgradeRuntime -match [regex]::Escape('.opencode/agents/reviewer.md') -and $upgradeRuntime -match 'rollback') 'OFFICIAL_UPGRADE_REVIEWER_HOST_TRANSACTION'

$orchestrator = Get-Content (Join-Path $repoRoot '.code-harness/agents/orchestrator.md') -Raw
$plainE2EModel = Get-Content (Join-Path $repoRoot '.github/scripts/task162-hotfix-task3/mock_openai_server.py') -Raw
Require ($orchestrator -match 'REVIEWER_UNAVAILABLE' -and $orchestrator -match 'MANUAL_ACTION_REQUIRED' -and $orchestrator -match 'HARD STOP') 'REAL_HARNESS_REVIEW_FAIL_CLOSED_CONTRACT'
Require ($orchestrator -match 'harness-review-reviewer' -and $orchestrator -match 'independent') 'REAL_HARNESS_REVIEW_INDEPENDENT_REVIEWER_BINDING'
Require ($plainE2EModel -notmatch 'Create semantic ChangeAnalysis proposal' -and $plainE2EModel -notmatch [regex]::Escape(".code-harness/runs/task3-plain-review/requests/change-analysis-proposal.json")) 'MAIN_AGENT_CHANGE_ANALYSIS_PROPOSAL_FORBIDDEN'

$task2E2E = Get-Content (Join-Path $repoRoot '.github/scripts/task164-release-blocker-task2-e2e.ps1') -Raw
foreach ($marker in @(
    'REVIEWER_REGISTRATION_MISSING_FAIL_CLOSED PASS',
    'REVIEWER_PRESENT_NOT_INVOKABLE_FAIL_CLOSED PASS',
    'REVIEWER_INVOCATION_FAILURE_FAIL_CLOSED PASS',
    'REVIEWER_CHILD_CANCEL_CRASH_FAIL_CLOSED PASS',
    'REVIEWER_MALFORMED_OUTPUT_FAIL_CLOSED PASS',
    'REVIEWER_NO_VALID_PROPOSAL_FAIL_CLOSED PASS',
    'MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN PASS',
    'REVIEWER_CHILD_PROPOSAL_RUNTIME_CERTIFY PASS',
    'REVIEWER_CHILD_PROPOSAL_RUNTIME_REJECT PASS',
    'REVIEW_OPTIONS_FROM_REVIEWER_CERTIFIED_ANALYSIS PASS',
    'REVIEW_NO_OPTIONS_FROM_REVIEWER_CERTIFIED_ANALYSIS PASS',
    'OFFICIAL_UPGRADE_REVIEWER_HOST_TRANSACTION PASS',
    'OFFICIAL_UPGRADE_REVIEWER_HOST_ROLLBACK PASS'
)) {
    Require ($task2E2E -match [regex]::Escape($marker)) ("MATRIX_" + (($marker -replace ' PASS$','') -replace '[^A-Z0-9_]','_'))
}

if ($failures.Count -eq 0) { throw 'Task 2 reverify RED unexpectedly green' }
Write-Output ('TASK164_TASK2_REVERIFY_EXPECTED_RED PASS failures=' + $failures.Count)
foreach ($failure in $failures) { Write-Output ("EXPECTED_RED $failure") }
exit 1
