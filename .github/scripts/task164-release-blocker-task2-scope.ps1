$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
Push-Location $repoRoot
try {
    $accepted = '51ad4cbc32008e388fc7a2564dbd5e518d04d9ae'
    $allowed = @(
        '.code-harness/bootstrap.md',
        '.code-harness/contracts/reviewer-host-contract.md',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/analysis_certify_canonical_162_hotfix_test.go',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/analysis_certify_test.go',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/analysis_command.go',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/certified_analysis_fixture_153_test.go',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/chain_edit_intent_authority_153_test.go',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/report.go',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/report_certified_153_test.go',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/report_certified_findings_164.go',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/report_test.go',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/review_precision_command.go',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/reviewer_authority_test.go',
        '.code-harness/tools-runtime/cmd/codea-dcep-tools/reviewer_findings_report_164_test.go',
        '.code-harness/tools-runtime/internal/reviewauthority/authority.go',
        '.code-harness/tools-runtime/internal/upgrade/reviewer_host_164.go',
        '.code-harness/tools-runtime/internal/upgrade/task164_reviewer_host_upgrade_test.go',
        '.code-harness/tools-runtime/internal/upgrade/upgrade.go',
        '.code-harness/tools/codea-reviewer-submit.ts',
        '.github/scripts/task164-release-blocker-task2-e2e.ps1',
        '.github/scripts/task164-release-blocker-task2-upgrade-e2e.ps1',
        '.github/scripts/task164-release-blocker-task2-plain-review-e2e.ps1',
        '.github/scripts/task164-release-blocker-task2-plain-review-server.py',
        '.github/scripts/task164-release-blocker-task2-gate-contract.py',
        '.github/scripts/task164-release-blocker-task2-scope.ps1',
        '.github/scripts/task164-release-blocker-task2-scope-self-test.ps1',
        '.github/scripts/task164-release-blocker-task2-session-cancel-e2e.ps1',
        '.github/scripts/task164-release-blocker-task2-red.ps1',
        '.github/scripts/task164-release-blocker-task2-reverify-red.ps1',
        '.github/scripts/task164-release-package.ps1',
        '.github/workflows/task164-release-blocker-task2-red.yml',
        '.github/workflows/task164-release-blocker-task2-reverify-red.yml',
        '.github/workflows/task164-release-blocker-task2.yml'
    )
    $changed = @(git diff --name-only "$accepted...HEAD")
    if ($LASTEXITCODE -ne 0) { throw 'cannot inspect Task 2 scope' }
    $unexpected = @($changed | Where-Object { $_ -notin $allowed })
    if ($unexpected.Count -gt 0) { throw "Task 2 scope expanded: $($unexpected -join ', ')" }
    $dirty = @(git diff --name-only HEAD)
    if ($LASTEXITCODE -ne 0 -or $dirty.Count -ne 0) { throw "tracked source changed during Task 2 CI: $($dirty -join ',')" }
    $head = (git rev-parse HEAD).Trim()
    if ([string]::IsNullOrWhiteSpace($env:GITHUB_SHA)) { throw 'GITHUB_SHA is required for exact HEAD verification' }
    if ($head -ne $env:GITHUB_SHA) { throw "exact HEAD mismatch: $head != $env:GITHUB_SHA" }
    Write-Output "TASK164_RELEASE_BLOCKER_TASK2_SCOPE PASS files=$($changed.Count)"
    Write-Output "TASK164_RELEASE_BLOCKER_TASK2_EXACT_HEAD PASS head=$head"
} finally {
    Pop-Location
}
