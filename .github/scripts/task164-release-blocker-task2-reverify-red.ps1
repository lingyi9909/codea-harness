$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$failures = [System.Collections.Generic.List[string]]::new()

function Require([bool]$Condition, [string]$Name) {
    if ($Condition) {
        Write-Output "$Name PASS"
        return
    }
    $failures.Add($Name)
}

$upgradeZip = Join-Path $repoRoot 'codea-harness-1.6.4-windows-x64-upgrade.zip'
if (-not (Test-Path $upgradeZip -PathType Leaf)) { throw "missing upgrade package: $upgradeZip" }
Add-Type -AssemblyName System.IO.Compression.FileSystem
$zip = [IO.Compression.ZipFile]::OpenRead($upgradeZip)
try { $entries = @($zip.Entries | ForEach-Object { $_.FullName.Replace('\','/') }) } finally { $zip.Dispose() }

Require (-not ($entries | Where-Object { $_ -like '.opencode/*' })) 'OFFICIAL_UPGRADE_NO_PRETRANSACTION_HOST_WRITE'
Require ($entries -contains '.code-harness-upgrade/host/.opencode/agents/reviewer.md') 'OFFICIAL_UPGRADE_REVIEWER_HOST_STAGED_SOURCE'
Require ($entries -contains '.code-harness-upgrade/host/.opencode/commands/harness-review-reviewer.md') 'OFFICIAL_UPGRADE_REVIEWER_COMMAND_STAGED_SOURCE'
Require ($entries -contains '.code-harness-upgrade/host/.opencode/tools/codea-reviewer-submit.ts') 'OFFICIAL_UPGRADE_REVIEWER_TOOL_STAGED_SOURCE'

$hostImpl = Get-Content (Join-Path $repoRoot '.code-harness/tools-runtime/internal/upgrade/reviewer_host_164.go') -Raw
$hostTests = Get-Content (Join-Path $repoRoot '.code-harness/tools-runtime/internal/upgrade/task164_reviewer_host_upgrade_test.go') -Raw
Require (
    $hostImpl -match 'prepareReviewerHostTransaction' -and
    $hostImpl -match 'func \(t \*reviewerHostTransaction\) apply' -and
    $hostImpl -match 'func \(t \*reviewerHostTransaction\) rollback' -and
    $hostImpl -match 'failAndRollbackWithReviewerHost' -and
    $hostImpl -match [regex]::Escape('.opencode/agents/reviewer.md')
) 'OFFICIAL_UPGRADE_REVIEWER_HOST_TRANSACTION'
Require (
    $hostTests -match 'Test164OfficialUpgradeInstallsReviewerHostTransactionally' -and
    $hostTests -match 'Test164OfficialUpgradeRejectsUnknownReviewerHostConflictBeforeFrameworkWrite' -and
    $hostTests -match 'Test164OfficialUpgradeRollsBackFrameworkAndPartialReviewerHostCommit'
) 'OFFICIAL_UPGRADE_REVIEWER_HOST_ROLLBACK_CONTRACT'

$orchestrator = Get-Content (Join-Path $repoRoot '.code-harness/agents/orchestrator.md') -Raw
$bootstrap = Get-Content (Join-Path $repoRoot '.code-harness/bootstrap.md') -Raw
$reviewerContract = Get-Content (Join-Path $repoRoot '.code-harness/contracts/reviewer-host-contract.md') -Raw
$activeReviewContract = $orchestrator + "`n" + $bootstrap + "`n" + $reviewerContract
Require (
    $activeReviewContract -match 'REVIEWER_UNAVAILABLE' -and
    $activeReviewContract -match 'MANUAL_ACTION_REQUIRED' -and
    $activeReviewContract -match 'HARD STOP'
) 'REAL_HARNESS_REVIEW_FAIL_CLOSED_CONTRACT'
Require (
    $orchestrator -match 'Reviewer\.analyze-change' -and
    $orchestrator -match 'Reviewer\.review-code' -and
    $bootstrap -match '独立 Reviewer child session' -and
    $reviewerContract -match 'independent' -and
    $reviewerContract -match 'reviewer.*subagent'
) 'REAL_HARNESS_REVIEW_INDEPENDENT_REVIEWER_BINDING'
Require (
    $reviewerContract -match 'Main Agent / Orchestrator MUST NOT perform Reviewer semantic work itself' -and
    $reviewerContract -match 'MUST NOT synthesize.*change-analysis-proposal\.json'
) 'MAIN_AGENT_CHANGE_ANALYSIS_PROPOSAL_FORBIDDEN'

$task2E2E = Get-Content (Join-Path $repoRoot '.github/scripts/task164-release-blocker-task2-e2e.ps1') -Raw
$cancelE2E = Get-Content (Join-Path $repoRoot '.github/scripts/task164-release-blocker-task2-session-cancel-e2e.ps1') -Raw

Require (
    $task2E2E -match [regex]::Escape('REVIEWER_UNAVAILABLE_FAIL_CLOSED PASS') -and
    $task2E2E -match [regex]::Escape("Remove-Item (Join-Path `$negative '.opencode/agents/reviewer.md')")
) 'MATRIX_REVIEWER_REGISTRATION_MISSING_FAIL_CLOSED'
Require ($task2E2E -match [regex]::Escape('REVIEWER_FILE_PRESENT_NOT_HOST_INVOKABLE_FAIL_CLOSED PASS')) 'MATRIX_REVIEWER_PRESENT_NOT_INVOKABLE_FAIL_CLOSED'
Require ($task2E2E -match [regex]::Escape('REVIEWER_INVOCATION_FAILURE_FAIL_CLOSED PASS')) 'MATRIX_REVIEWER_INVOCATION_FAILURE_FAIL_CLOSED'
Require (
    $cancelE2E -match [regex]::Escape('REVIEWER_CHILD_CANCEL_CRASH_FAIL_CLOSED PASS') -and
    $cancelE2E -match [regex]::Escape("foreach(`$authority in @('analysis/change-analysis.json','analysis/change-analysis.cert.json','analysis/review-options.json','analysis/review-units.json','analysis/rule-dispatch.json','analysis/certified-findings.json','review.md'))")
) 'MATRIX_REVIEWER_CHILD_CANCEL_CRASH_FAIL_CLOSED'
Require ($task2E2E -match [regex]::Escape('REVIEWER_MALFORMED_OUTPUT_FAIL_CLOSED PASS')) 'MATRIX_REVIEWER_MALFORMED_OUTPUT_FAIL_CLOSED'
Require ($task2E2E -match [regex]::Escape('REVIEWER_NO_VALID_PROPOSAL_FAIL_CLOSED PASS')) 'MATRIX_REVIEWER_NO_VALID_PROPOSAL_FAIL_CLOSED'
Require ($task2E2E -match [regex]::Escape('MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN PASS')) 'MATRIX_MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN'
Require ($task2E2E -match [regex]::Escape('REVIEWER_PROPOSAL_RUNTIME_CERTIFICATION PASS')) 'MATRIX_REVIEWER_CHILD_PROPOSAL_RUNTIME_CERTIFY'
Require (
    $task2E2E -match 'Reviewer failure incorrectly certified analysis' -and
    $task2E2E -match [regex]::Escape('REVIEWER_MALFORMED_OUTPUT_FAIL_CLOSED PASS') -and
    $task2E2E -match [regex]::Escape('REVIEWER_NO_VALID_PROPOSAL_FAIL_CLOSED PASS')
) 'MATRIX_REVIEWER_CHILD_PROPOSAL_RUNTIME_REJECT'
Require ($task2E2E -match [regex]::Escape('REVIEWER_VALID_PROPOSAL_REVIEW_OPTIONS PASS')) 'MATRIX_REVIEW_OPTIONS_FROM_REVIEWER_CERTIFIED_ANALYSIS'
Require (
    $task2E2E -match [regex]::Escape("'analysis/review-options.json'") -and
    $task2E2E -match [regex]::Escape('Reviewer failure incorrectly certified analysis')
) 'MATRIX_REVIEW_NO_OPTIONS_FROM_REVIEWER_CERTIFIED_ANALYSIS'
Require (
    $hostImpl -match 'prepareReviewerHostTransaction' -and
    $hostImpl -match 'host\.apply\(' -or
    $hostImpl -match 'func \(t \*reviewerHostTransaction\) apply'
) 'MATRIX_OFFICIAL_UPGRADE_REVIEWER_HOST_TRANSACTION'
Require ($hostTests -match 'Test164OfficialUpgradeRollsBackFrameworkAndPartialReviewerHostCommit') 'MATRIX_OFFICIAL_UPGRADE_REVIEWER_HOST_ROLLBACK'

if ($failures.Count -gt 0) {
    foreach ($failure in $failures) { Write-Output ("TASK164_TASK2_REVERIFY_FAIL $failure") }
    throw ('Task 2 reverify failed checks=' + $failures.Count)
}

Write-Output 'TASK164_TASK2_REVERIFY PASS'