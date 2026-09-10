$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$failures = [System.Collections.Generic.List[string]]::new()
function Require([bool]$Condition, [string]$Name) { if ($Condition) { Write-Output "$Name PASS"; return }; $failures.Add($Name) }

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
Require ($hostImpl -match 'prepareReviewerHostTransaction' -and $hostImpl -match 'func \(t \*reviewerHostTransaction\) apply' -and $hostImpl -match 'func \(t \*reviewerHostTransaction\) rollback' -and $hostImpl -match 'failAndRollbackWithReviewerHost') 'OFFICIAL_UPGRADE_REVIEWER_HOST_TRANSACTION'
Require ($hostTests -match 'Test164OfficialUpgradeInstallsReviewerHostTransactionally' -and $hostTests -match 'Test164OfficialUpgradeRejectsUnknownReviewerHostConflictBeforeFrameworkWrite' -and $hostTests -match 'Test164OfficialUpgradeRollsBackFrameworkAndPartialReviewerHostCommit') 'OFFICIAL_UPGRADE_REVIEWER_HOST_ROLLBACK_CONTRACT'

$orchestrator = Get-Content (Join-Path $repoRoot '.code-harness/agents/orchestrator.md') -Raw
$bootstrap = Get-Content (Join-Path $repoRoot '.code-harness/bootstrap.md') -Raw
$reviewerContract = Get-Content (Join-Path $repoRoot '.code-harness/contracts/reviewer-host-contract.md') -Raw
$activeReviewContract = $orchestrator + "`n" + $bootstrap + "`n" + $reviewerContract
Require ($activeReviewContract -match 'REVIEWER_UNAVAILABLE' -and $activeReviewContract -match 'MANUAL_ACTION_REQUIRED' -and $activeReviewContract -match 'HARD STOP') 'REAL_HARNESS_REVIEW_FAIL_CLOSED_CONTRACT'
Require ($orchestrator -match 'Reviewer\.analyze-change' -and $orchestrator -match 'Reviewer\.review-code' -and $bootstrap -match '独立 Reviewer child session' -and $reviewerContract -match 'independent' -and $reviewerContract -match 'reviewer.*subagent') 'REAL_HARNESS_REVIEW_INDEPENDENT_REVIEWER_BINDING'
Require ($reviewerContract -match 'Main Agent / Orchestrator MUST NOT perform Reviewer semantic work itself' -and $reviewerContract -match 'MUST NOT synthesize.*change-analysis-proposal\.json') 'MAIN_AGENT_CHANGE_ANALYSIS_PROPOSAL_FORBIDDEN'
Require ($reviewerContract -match 'opencode export' -and $reviewerContract -match 'receipt.*not.*authority' -and $reviewerContract -match 'completed.*submission') 'REVIEWER_HOST_ATTESTATION_BOUNDARY_CONTRACT'
$authority = Get-Content (Join-Path $repoRoot '.code-harness/tools-runtime/internal/reviewauthority/authority.go') -Raw
Require ($authority -match 'exportOpenCodeSession' -and $authority -match 'verifySessionAttestation' -and $authority -match 'opencode.*export' -and $authority -match 'codeareviewersubmit') 'REVIEWER_HOST_ATTESTATION_RUNTIME_ENFORCED'

$task2E2E = Get-Content (Join-Path $repoRoot '.github/scripts/task164-release-blocker-task2-e2e.ps1') -Raw
$cancelE2E = Get-Content (Join-Path $repoRoot '.github/scripts/task164-release-blocker-task2-session-cancel-e2e.ps1') -Raw
$upgradeE2E = Get-Content (Join-Path $repoRoot '.github/scripts/task164-release-blocker-task2-upgrade-e2e.ps1') -Raw
$plainE2E = Get-Content (Join-Path $repoRoot '.github/scripts/task164-release-blocker-task2-plain-review-e2e.ps1') -Raw
$plainServer = Get-Content (Join-Path $repoRoot '.github/scripts/task164-release-blocker-task2-plain-review-server.py') -Raw
$workflow = Get-Content (Join-Path $repoRoot '.github/workflows/task164-release-blocker-task2.yml') -Raw
Require ($task2E2E -match [regex]::Escape('REVIEWER_UNAVAILABLE_FAIL_CLOSED PASS')) 'MATRIX_REVIEWER_REGISTRATION_MISSING_FAIL_CLOSED'
Require ($task2E2E -match [regex]::Escape('REVIEWER_FILE_PRESENT_NOT_HOST_INVOKABLE_FAIL_CLOSED PASS')) 'MATRIX_REVIEWER_PRESENT_NOT_INVOKABLE_FAIL_CLOSED'
Require ($task2E2E -match [regex]::Escape('REVIEWER_INVOCATION_FAILURE_FAIL_CLOSED PASS')) 'MATRIX_REVIEWER_INVOCATION_FAILURE_FAIL_CLOSED'
Require ($cancelE2E -match [regex]::Escape('REVIEWER_CHILD_CANCEL_CRASH_FAIL_CLOSED PASS')) 'MATRIX_REVIEWER_CHILD_CANCEL_CRASH_FAIL_CLOSED'
Require ($task2E2E -match [regex]::Escape('REVIEWER_MALFORMED_OUTPUT_FAIL_CLOSED PASS')) 'MATRIX_REVIEWER_MALFORMED_OUTPUT_FAIL_CLOSED'
Require ($task2E2E -match [regex]::Escape('REVIEWER_NO_VALID_PROPOSAL_FAIL_CLOSED PASS')) 'MATRIX_REVIEWER_NO_VALID_PROPOSAL_FAIL_CLOSED'
Require ($task2E2E -match [regex]::Escape('MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN PASS')) 'MATRIX_MAIN_AGENT_REVIEWER_FALLBACK_FORBIDDEN'
Require ($task2E2E -match [regex]::Escape('REVIEWER_PROPOSAL_RUNTIME_CERTIFICATION PASS')) 'MATRIX_REVIEWER_CHILD_PROPOSAL_RUNTIME_CERTIFY'
Require ($task2E2E -match [regex]::Escape('REVIEWER_VALID_PROPOSAL_REVIEW_OPTIONS PASS')) 'MATRIX_REVIEW_OPTIONS_FROM_REVIEWER_CERTIFIED_ANALYSIS'
Require ($upgradeE2E -match 'artifactId\s*=\s*10004022218' -and $upgradeE2E -match [regex]::Escape('TASK164_PACKAGED_163_TO_164_REVIEWER_HOST_UPGRADE PASS') -and $upgradeE2E -match 'opencode agent list') 'PACKAGED_EXACT_163_TO_164_REVIEWER_HOST_RESOLVABLE'
Require ($upgradeE2E -match [regex]::Escape('TASK164_PACKAGED_163_TO_164_REVIEWER_HOST_ROLLBACK PASS') -and $upgradeE2E -match 'rollbackPerformed' -and $upgradeE2E -match 'icacls') 'PACKAGED_EXACT_163_TO_164_REVIEWER_HOST_ROLLBACK'
Require ($workflow -match [regex]::Escape('task164-release-blocker-task2-upgrade-e2e.ps1')) 'PACKAGED_UPGRADE_E2E_WORKFLOW_BOUND'
Require ($plainE2E -match "'harness review'" -and $plainServer -match 'TASK164_PLAIN_STAGE_BEGIN PASS' -and $plainServer -match 'TASK164_PLAIN_STAGE_SNAPSHOT PASS' -and $plainServer -match 'TASK164_PLAIN_STAGE_REVIEWER PASS' -and $plainServer -match 'TASK164_PLAIN_STAGE_RUNTIME_CERTIFY PASS' -and $plainServer -match 'TASK164_PLAIN_STAGE_REVIEW_OPTIONS PASS') 'PACKAGED_TOP_LEVEL_HARNESS_REVIEW_CHAIN'
Require ($plainServer -match 'respond_tool\(body, "task"' -and $plainServer -match '"subagent_type": "reviewer"' -and $plainE2E -match '"subagent_type":"reviewer"') 'PACKAGED_TOP_LEVEL_INDEPENDENT_REVIEWER_CHILD'
Require ($plainServer -notmatch 'opencode run --command harness-review-reviewer') 'PACKAGED_TOP_LEVEL_NO_NESTED_OPENCODE_REVIEWER_PROCESS'
Require ($plainE2E -match [regex]::Escape('TASK164_PACKAGED_PLAIN_HARNESS_REVIEW_DISABLED_FAIL_CLOSED PASS') -and $plainE2E -match 'Assert-ZeroDownstreamAuthority') 'PACKAGED_TOP_LEVEL_REVIEWER_DISABLED_HARD_STOP'
Require ($plainE2E -match [regex]::Escape('REVIEWER_FILE_PRESENT_NOT_HOST_INVOKABLE_SAME_RUN_HARD_STOP PASS') -and $plainE2E -match 'mode: definitely-invalid') 'REVIEWER_PRESENT_NOT_INVOKABLE_SAME_RUN_ZERO_AUTHORITY'
Require ($plainE2E -match [regex]::Escape('MAIN_AGENT_FORGED_REVIEWER_RECEIPT_RUNTIME_REJECT PASS') -and $plainE2E -match 'ses_forged_main_agent' -and $plainE2E -match 'analysis certify') 'MAIN_AGENT_FORGED_REVIEWER_RECEIPT_RUNTIME_REJECT'
Require ($workflow -match [regex]::Escape('task164-release-blocker-task2-plain-review-e2e.ps1')) 'PACKAGED_TOP_LEVEL_REVIEW_WORKFLOW_BOUND'

if ($failures.Count -gt 0) { foreach ($failure in $failures) { Write-Output ("TASK164_TASK2_REVERIFY_FAIL $failure") }; throw ('Task 2 reverify failed checks=' + $failures.Count) }
Write-Output 'TASK164_TASK2_REVERIFY PASS'
